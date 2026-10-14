// Package api serves the dashboard, public catalog, Auth.js adapter and payment
// webhook endpoints.
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/billing"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/bus"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/catalog"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/config"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/httpx"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/payments"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/secret"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/session"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
)

const maxKeysPerUser = 50

type Server struct {
	St       *store.Store
	Sess     *session.Auth
	Ledger   *billing.Ledger
	Catalog  *catalog.Catalog
	Payments *payments.Registry
	Bus      *bus.Bus
	Cfg      *config.Config
	Log      *slog.Logger
}

func (s *Server) Register(mux *http.ServeMux) {
	// Auth.js adapter (server-to-server).
	in := s.Sess.Internal
	mux.HandleFunc("POST /internal/auth/users", in(s.authCreateUser))
	mux.HandleFunc("GET /internal/auth/users/{id}", in(s.authGetUser))
	mux.HandleFunc("PATCH /internal/auth/users/{id}", in(s.authUpdateUser))
	mux.HandleFunc("GET /internal/auth/user-by-email", in(s.authUserByEmail))
	mux.HandleFunc("GET /internal/auth/user-by-account", in(s.authUserByAccount))
	mux.HandleFunc("POST /internal/auth/accounts", in(s.authLinkAccount))
	mux.HandleFunc("POST /internal/auth/verification-tokens", in(s.authCreateToken))
	mux.HandleFunc("POST /internal/auth/verification-tokens/use", in(s.authUseToken))

	// Public catalog for the website.
	mux.HandleFunc("GET /api/public/models", s.publicModels)
	mux.HandleFunc("GET /api/public/models/{slug...}", s.publicModel)
	mux.HandleFunc("GET /api/public/config", s.publicConfig)

	// Dashboard.
	u := s.Sess.User
	mux.HandleFunc("GET /api/me", u(s.me))
	mux.HandleFunc("PATCH /api/me", u(s.updateMe))
	mux.HandleFunc("GET /api/keys", u(s.listKeys))
	mux.HandleFunc("POST /api/keys", u(s.createKey))
	mux.HandleFunc("PATCH /api/keys/{id}", u(s.updateKey))
	mux.HandleFunc("POST /api/keys/{id}/rotate", u(s.rotateKey))
	mux.HandleFunc("DELETE /api/keys/{id}", u(s.deleteKey))
	mux.HandleFunc("GET /api/usage/daily", u(s.usageDaily))
	mux.HandleFunc("GET /api/usage/models", u(s.usageModels))
	mux.HandleFunc("GET /api/activity", u(s.activity))
	mux.HandleFunc("GET /api/billing/methods", u(s.billingMethods))
	mux.HandleFunc("POST /api/billing/checkout", u(s.checkout))
	mux.HandleFunc("GET /api/billing/payments", u(s.listPayments))
	mux.HandleFunc("GET /api/billing/payments/{id}", u(s.getPayment))
	mux.HandleFunc("GET /api/billing/ledger", u(s.ledger))

	// Payment provider webhooks.
	mux.HandleFunc("POST /webhooks/{provider}", s.webhook)
}

func (s *Server) isAdminEmail(email string) bool {
	return slices.Contains(s.Cfg.AdminEmails, strings.ToLower(strings.TrimSpace(email)))
}

// storeErr maps store errors to HTTP responses.
func storeErr(w http.ResponseWriter, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpx.Error(w, 404, "not found")
	case errors.Is(err, store.ErrConflict):
		httpx.Error(w, 409, "already exists")
	default:
		log.Error("store error", "err", err)
		httpx.Error(w, 500, "internal error")
	}
}

// ---------------------------------------------------------------------------
// Auth.js adapter
// ---------------------------------------------------------------------------

type adapterUser struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	Name          string     `json:"name"`
	Image         string     `json:"image"`
	EmailVerified *time.Time `json:"emailVerified"`
	Role          string     `json:"role"`
}

func toAdapter(u *store.User) adapterUser {
	return adapterUser{ID: u.ID, Email: u.Email, Name: u.Name, Image: u.Image, EmailVerified: u.EmailVerified, Role: u.Role}
}

func (s *Server) authCreateUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email         string     `json:"email"`
		Name          string     `json:"name"`
		Image         string     `json:"image"`
		EmailVerified *time.Time `json:"emailVerified"`
	}
	if err := httpx.DecodeJSON(r, 64<<10, &in); err != nil || !strings.Contains(in.Email, "@") {
		httpx.Error(w, 400, "invalid user")
		return
	}
	role := "user"
	if s.isAdminEmail(in.Email) {
		role = "admin"
	}
	bonus := s.Catalog.Snapshot().Settings.SignupBonus
	u, err := s.St.CreateUser(r.Context(), in.Email, in.Name, in.Image, in.EmailVerified, role, bonus)
	if errors.Is(err, store.ErrConflict) {
		u, err = s.St.GetUserByEmail(r.Context(), in.Email)
	}
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	httpx.JSON(w, 200, toAdapter(u))
}

func (s *Server) authGetUser(w http.ResponseWriter, r *http.Request) {
	u, err := s.St.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.JSON(w, 200, nil)
			return
		}
		storeErr(w, s.Log, err)
		return
	}
	httpx.JSON(w, 200, toAdapter(u))
}

func (s *Server) authUserByEmail(w http.ResponseWriter, r *http.Request) {
	u, err := s.St.GetUserByEmail(r.Context(), r.URL.Query().Get("email"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.JSON(w, 200, nil)
		return
	}
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	if u.Role != "admin" && s.isAdminEmail(u.Email) {
		role := "admin"
		if nu, err := s.St.UpdateUser(r.Context(), u.ID, store.UserPatch{Role: &role}); err == nil {
			u = nu
		}
	}
	httpx.JSON(w, 200, toAdapter(u))
}

func (s *Server) authUserByAccount(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	u, err := s.St.GetUserByAccount(r.Context(), q.Get("provider"), q.Get("providerAccountId"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.JSON(w, 200, nil)
		return
	}
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	httpx.JSON(w, 200, toAdapter(u))
}

func (s *Server) authUpdateUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name          *string    `json:"name"`
		Image         *string    `json:"image"`
		Email         *string    `json:"email"`
		EmailVerified *time.Time `json:"emailVerified"`
	}
	if err := httpx.DecodeJSON(r, 64<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	u, err := s.St.UpdateUser(r.Context(), r.PathValue("id"), store.UserPatch{
		Name: in.Name, Image: in.Image, Email: in.Email, EmailVerified: in.EmailVerified,
	})
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	s.Sess.Invalidate(u.ID)
	httpx.JSON(w, 200, toAdapter(u))
}

func (s *Server) authLinkAccount(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID            string `json:"userId"`
		Provider          string `json:"provider"`
		ProviderAccountID string `json:"providerAccountId"`
		Type              string `json:"type"`
	}
	if err := httpx.DecodeJSON(r, 64<<10, &in); err != nil || in.UserID == "" || in.Provider == "" || in.ProviderAccountID == "" {
		httpx.Error(w, 400, "invalid account")
		return
	}
	if err := s.St.LinkAccount(r.Context(), in.UserID, in.Provider, in.ProviderAccountID, in.Type); err != nil {
		storeErr(w, s.Log, err)
		return
	}
	httpx.JSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) authCreateToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Identifier string    `json:"identifier"`
		Token      string    `json:"token"`
		Expires    time.Time `json:"expires"`
	}
	if err := httpx.DecodeJSON(r, 64<<10, &in); err != nil || in.Identifier == "" || in.Token == "" {
		httpx.Error(w, 400, "invalid token")
		return
	}
	// Auth.js already hashes the token with the secret; hash again so a DB leak
	// cannot be replayed.
	if err := s.St.CreateVerificationToken(r.Context(), in.Identifier, secret.HashKey(in.Token), in.Expires); err != nil {
		storeErr(w, s.Log, err)
		return
	}
	httpx.JSON(w, 200, in)
}

func (s *Server) authUseToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Identifier string `json:"identifier"`
		Token      string `json:"token"`
	}
	if err := httpx.DecodeJSON(r, 64<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid token")
		return
	}
	exp, err := s.St.UseVerificationToken(r.Context(), in.Identifier, secret.HashKey(in.Token))
	if errors.Is(err, store.ErrNotFound) {
		httpx.JSON(w, 200, nil)
		return
	}
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"identifier": in.Identifier, "token": in.Token, "expires": exp})
}

// ---------------------------------------------------------------------------
// Public catalog
// ---------------------------------------------------------------------------

type PublicModel struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Author          string    `json:"author"`
	Description     string    `json:"description"`
	ContextLength   int       `json:"context_length"`
	MaxOutputTokens int       `json:"max_output_tokens"`
	InputPrice      string    `json:"input_price_per_m"`  // USD per 1M input tokens
	OutputPrice     string    `json:"output_price_per_m"` // USD per 1M output tokens
	RequestFee      string    `json:"request_fee"`        // USD per request
	Modalities      []string  `json:"modalities"`
	SupportsTools   bool      `json:"supports_tools"`
	Featured        bool      `json:"featured"`
	Free            bool      `json:"free"`
	CreatedAt       time.Time `json:"created_at"`
}

func toPublic(e *catalog.Entry) PublicModel {
	m := e.Model
	return PublicModel{
		ID: m.Slug, Name: m.Name, Author: m.Author, Description: m.Description, ContextLength: m.ContextLength,
		MaxOutputTokens: m.MaxOutputTokens, InputPrice: billing.FormatUSD(e.Price.InputPerM),
		OutputPrice: billing.FormatUSD(e.Price.OutputPerM), RequestFee: billing.FormatUSD(e.Price.RequestFee),
		Modalities: m.Modalities, SupportsTools: m.SupportsTools, Featured: m.Featured, Free: e.IsFree(), CreatedAt: m.CreatedAt,
	}
}

func (s *Server) publicModels(w http.ResponseWriter, r *http.Request) {
	snap := s.Catalog.Snapshot()
	out := make([]PublicModel, 0, len(snap.Models))
	for _, e := range snap.Models {
		out = append(out, toPublic(e))
	}
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	httpx.JSON(w, 200, map[string]any{"data": out})
}

func (s *Server) publicModel(w http.ResponseWriter, r *http.Request) {
	e, ok := s.Catalog.Snapshot().Lookup(r.PathValue("slug"))
	if !ok {
		httpx.Error(w, 404, "model not found")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	httpx.JSON(w, 200, toPublic(e))
}

func (s *Server) publicConfig(w http.ResponseWriter, r *http.Request) {
	st := s.Catalog.Snapshot().Settings
	httpx.JSON(w, 200, map[string]any{
		"announcement":    st.Announcement,
		"maintenance":     st.MaintenanceMode,
		"min_topup_cents": st.MinTopupCents,
		"max_topup_cents": st.MaxTopupCents,
		"bdt_per_usd":     st.BDTPerUSD,
		"request_fee":     billing.FormatUSD(st.DefaultRequestFee),
		"api_base_url":    s.Cfg.PublicURL + "/v1",
	})
}
