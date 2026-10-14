// Package admin serves the admin panel API. Every route requires a session
// whose user has the admin role, and every mutation is written to the audit log.
package admin

import (
	"errors"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/billing"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/bus"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/catalog"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/httpx"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/secret"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/session"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/upstream"
)

type Server struct {
	St      *store.Store
	Sess    *session.Auth
	Catalog *catalog.Catalog
	Ledger  *billing.Ledger
	Bus     *bus.Bus
	Box     *secret.Box
	Log     *slog.Logger
}

func (s *Server) Register(mux *http.ServeMux) {
	a := s.Sess.Admin
	mux.HandleFunc("GET /api/admin/overview", a(s.overview))
	mux.HandleFunc("GET /api/admin/users", a(s.listUsers))
	mux.HandleFunc("GET /api/admin/users/{id}", a(s.getUser))
	mux.HandleFunc("PATCH /api/admin/users/{id}", a(s.updateUser))
	mux.HandleFunc("POST /api/admin/users/{id}/credit", a(s.creditUser))
	mux.HandleFunc("DELETE /api/admin/keys/{id}", a(s.deleteKey))
	mux.HandleFunc("PATCH /api/admin/keys/{id}", a(s.updateKey))

	mux.HandleFunc("GET /api/admin/models", a(s.listModels))
	mux.HandleFunc("POST /api/admin/models", a(s.createModel))
	mux.HandleFunc("GET /api/admin/models/{id}", a(s.getModel))
	mux.HandleFunc("PUT /api/admin/models/{id}", a(s.updateModel))
	mux.HandleFunc("DELETE /api/admin/models/{id}", a(s.deleteModel))
	mux.HandleFunc("POST /api/admin/models/bulk", a(s.bulkModels))
	mux.HandleFunc("POST /api/admin/models/reprice", a(s.reprice))
	mux.HandleFunc("GET /api/admin/upstream-models", a(s.listUpstream))
	mux.HandleFunc("POST /api/admin/upstream-models/publish", a(s.publish))

	mux.HandleFunc("GET /api/admin/providers", a(s.listProviders))
	mux.HandleFunc("POST /api/admin/providers", a(s.createProvider))
	mux.HandleFunc("PATCH /api/admin/providers/{id}", a(s.updateProvider))
	mux.HandleFunc("DELETE /api/admin/providers/{id}", a(s.deleteProvider))
	mux.HandleFunc("POST /api/admin/providers/{id}/sync", a(s.syncProvider))

	mux.HandleFunc("GET /api/admin/payments", a(s.listPayments))
	mux.HandleFunc("GET /api/admin/usage", a(s.listUsage))
	mux.HandleFunc("GET /api/admin/settings", a(s.getSettings))
	mux.HandleFunc("PUT /api/admin/settings", a(s.putSettings))
	mux.HandleFunc("GET /api/admin/audit", a(s.listAudit))
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpx.Error(w, 404, "not found")
	case errors.Is(err, store.ErrConflict):
		httpx.Error(w, 409, "conflict: already exists or still in use")
	default:
		s.Log.Error("admin store error", "err", err)
		httpx.Error(w, 500, "internal error")
	}
}

func (s *Server) audit(r *http.Request, u *store.User, action, target string, detail map[string]any) {
	s.St.Audit(r.Context(), u.ID, u.Email, action, target, detail)
}

func (s *Server) catalogChanged(r *http.Request) {
	if err := s.Catalog.Reload(r.Context()); err != nil {
		s.Log.Warn("catalog reload failed", "err", err)
	}
	s.Bus.Publish(r.Context(), bus.KindCatalog, "")
}

func pathInt(r *http.Request, name string) (int64, bool) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return v, err == nil
}

// ---------------------------------------------------------------------------
// Overview
// ---------------------------------------------------------------------------

func (s *Server) overview(w http.ResponseWriter, r *http.Request, _ *store.User) {
	days := httpx.QueryInt(r, "days", 30, 1, 365)
	o, err := s.St.Overview(r.Context(), days)
	if err != nil {
		s.fail(w, err)
		return
	}
	daily, err := s.St.DailyUsage(r.Context(), "", days)
	if err != nil {
		s.fail(w, err)
		return
	}
	models, err := s.St.ModelUsage(r.Context(), "", days, 10)
	if err != nil {
		s.fail(w, err)
		return
	}
	users, err := s.St.TopUsers(r.Context(), days, 10)
	if err != nil {
		s.fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{
		"overview": o, "daily": daily, "top_models": models, "top_users": users,
		"live": map[string]any{"models_enabled": len(s.Catalog.Snapshot().Models), "billing_queue": s.Ledger.QueueLen()},
	})
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request, _ *store.User) {
	limit := httpx.QueryInt(r, "limit", 50, 1, 200)
	offset := httpx.QueryInt(r, "offset", 0, 0, 10_000_000)
	users, total, err := s.St.ListUsers(r.Context(), r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		s.fail(w, err)
		return
	}
	if users == nil {
		users = []store.User{}
	}
	httpx.JSON(w, 200, map[string]any{"data": users, "total": total})
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request, _ *store.User) {
	id := r.PathValue("id")
	u, err := s.St.GetUser(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	keys, err := s.St.ListKeys(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	ledger, err := s.St.ListLedger(r.Context(), id, 50)
	if err != nil {
		s.fail(w, err)
		return
	}
	pays, _, err := s.St.ListPayments(r.Context(), id, "", 50, 0)
	if err != nil {
		s.fail(w, err)
		return
	}
	daily, err := s.St.DailyUsage(r.Context(), id, 30)
	if err != nil {
		s.fail(w, err)
		return
	}
	avail, _ := s.Ledger.Available(r.Context(), id)
	if ledger == nil {
		ledger = []store.LedgerEntry{}
	}
	httpx.JSON(w, 200, map[string]any{"user": u, "available_balance": avail, "keys": keys, "ledger": ledger, "payments": pays, "daily": daily})
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, admin *store.User) {
	var in struct {
		Role    *string `json:"role"`
		Banned  *bool   `json:"banned"`
		Name    *string `json:"name"`
		Country *string `json:"country"`
	}
	if err := httpx.DecodeJSON(r, 16<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	if in.Role != nil && *in.Role != "user" && *in.Role != "admin" {
		httpx.Error(w, 400, "role must be user or admin")
		return
	}
	id := r.PathValue("id")
	if id == admin.ID && ((in.Role != nil && *in.Role != "admin") || (in.Banned != nil && *in.Banned)) {
		httpx.Error(w, 400, "you cannot demote or ban yourself")
		return
	}
	u, err := s.St.UpdateUser(r.Context(), id, store.UserPatch{Role: in.Role, Banned: in.Banned, Name: in.Name, Country: in.Country})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.Sess.Invalidate(id)
	s.Bus.Publish(r.Context(), bus.KindUser, id)
	s.audit(r, admin, "user.update", id, map[string]any{"role": in.Role, "banned": in.Banned})
	httpx.JSON(w, 200, map[string]any{"user": u})
}

func (s *Server) creditUser(w http.ResponseWriter, r *http.Request, admin *store.User) {
	var in struct {
		AmountUSD float64 `json:"amount_usd"`
		Note      string  `json:"note"`
	}
	if err := httpx.DecodeJSON(r, 16<<10, &in); err != nil || in.AmountUSD == 0 || math.Abs(in.AmountUSD) > 1e6 || math.IsNaN(in.AmountUSD) {
		httpx.Error(w, 400, "amount_usd must be a non-zero number up to 1,000,000")
		return
	}
	id := r.PathValue("id")
	amount := int64(math.Round(in.AmountUSD * float64(billing.NanoPerUSD)))
	bal, err := s.St.AdjustBalance(r.Context(), id, amount, "admin_adjustment", "", admin.Email, strings.TrimSpace(in.Note))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.Bus.Publish(r.Context(), bus.KindUser, id)
	s.audit(r, admin, "user.credit", id, map[string]any{"amount_usd": in.AmountUSD, "note": in.Note})
	httpx.JSON(w, 200, map[string]any{"balance": bal})
}

func (s *Server) deleteKey(w http.ResponseWriter, r *http.Request, admin *store.User) {
	hash, err := s.St.DeleteKey(r.Context(), "", r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.Bus.Publish(r.Context(), bus.KindKey, hash)
	s.audit(r, admin, "key.delete", r.PathValue("id"), nil)
	httpx.JSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) updateKey(w http.ResponseWriter, r *http.Request, admin *store.User) {
	var in struct {
		Disabled *bool `json:"disabled"`
		RPMLimit *int  `json:"rpm_limit"`
	}
	if err := httpx.DecodeJSON(r, 16<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	k, hash, err := s.St.UpdateKey(r.Context(), "", r.PathValue("id"), store.KeyPatch{Disabled: in.Disabled, RPMLimit: in.RPMLimit})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.Bus.Publish(r.Context(), bus.KindKey, hash)
	s.audit(r, admin, "key.update", k.ID, map[string]any{"disabled": in.Disabled, "rpm_limit": in.RPMLimit})
	httpx.JSON(w, 200, map[string]any{"key": k})
}

// ---------------------------------------------------------------------------
// Models
// ---------------------------------------------------------------------------

var slugRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,127}$`)

func (s *Server) listModels(w http.ResponseWriter, r *http.Request, _ *store.User) {
	ms, err := s.St.ListModels(r.Context(), true)
	if err != nil {
		s.fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"data": ms, "settings": s.Catalog.Snapshot().Settings})
}

func (s *Server) getModel(w http.ResponseWriter, r *http.Request, _ *store.User) {
	id, ok := pathInt(r, "id")
	if !ok {
		httpx.Error(w, 400, "invalid id")
		return
	}
	m, err := s.St.GetModel(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	httpx.JSON(w, 200, m)
}

// modelInput uses USD decimal strings for prices to avoid float rounding.
type modelInput struct {
	Slug             string   `json:"slug"`
	Name             string   `json:"name"`
	Author           string   `json:"author"`
	Description      string   `json:"description"`
	ProviderID       int64    `json:"provider_id"`
	UpstreamID       string   `json:"upstream_id"`
	FallbackUpstream []string `json:"fallback_upstream"`
	ContextLength    int      `json:"context_length"`
	MaxOutputTokens  int      `json:"max_output_tokens"`
	PricingMode      string   `json:"pricing_mode"`
	MarkupBps        *int     `json:"markup_bps"`
	InputPricePerM   string   `json:"input_price_per_m"`  // USD
	OutputPricePerM  string   `json:"output_price_per_m"` // USD
	RequestFee       *string  `json:"request_fee"`        // USD, null = platform default
	Modalities       []string `json:"modalities"`
	SupportsTools    bool     `json:"supports_tools"`
	Enabled          bool     `json:"enabled"`
	Featured         bool     `json:"featured"`
	SortOrder        int      `json:"sort_order"`
}

func (in *modelInput) toModel() (*store.Model, string) {
	in.Slug = strings.TrimSpace(in.Slug)
	if !slugRe.MatchString(in.Slug) {
		return nil, "slug may contain letters, digits and . _ : / @ - (max 128 chars)"
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, "name is required"
	}
	if in.ProviderID == 0 || strings.TrimSpace(in.UpstreamID) == "" {
		return nil, "provider and upstream model are required"
	}
	if in.PricingMode != "auto" && in.PricingMode != "manual" {
		in.PricingMode = "manual"
	}
	if in.MarkupBps != nil && (*in.MarkupBps < -10000 || *in.MarkupBps > 1_000_000) {
		return nil, "markup out of range"
	}
	if in.MaxOutputTokens < 0 || in.ContextLength < 0 {
		return nil, "token limits must be positive"
	}
	inP, err1 := billing.ParseUSD(in.InputPricePerM)
	outP, err2 := billing.ParseUSD(in.OutputPricePerM)
	if err1 != nil || err2 != nil {
		return nil, "prices must be non-negative decimal USD amounts"
	}
	var fee *int64
	if in.RequestFee != nil && strings.TrimSpace(*in.RequestFee) != "" {
		f, err := billing.ParseUSD(*in.RequestFee)
		if err != nil {
			return nil, "request fee must be a non-negative decimal USD amount"
		}
		fee = &f
	}
	var fallbacks []string
	for _, f := range in.FallbackUpstream {
		if f = strings.TrimSpace(f); f != "" {
			fallbacks = append(fallbacks, f)
		}
	}
	if len(in.Modalities) == 0 {
		in.Modalities = []string{"text"}
	}
	return &store.Model{
		Slug: in.Slug, Name: strings.TrimSpace(in.Name), Author: strings.TrimSpace(in.Author), Description: in.Description,
		ProviderID: in.ProviderID, UpstreamID: strings.TrimSpace(in.UpstreamID), FallbackUpstream: fallbacks,
		ContextLength: in.ContextLength, MaxOutputTokens: in.MaxOutputTokens, PricingMode: in.PricingMode,
		MarkupBps: in.MarkupBps, InputPrice: inP, OutputPrice: outP, RequestFee: fee, Modalities: in.Modalities,
		SupportsTools: in.SupportsTools, Enabled: in.Enabled, Featured: in.Featured, SortOrder: in.SortOrder,
	}, ""
}

func (s *Server) createModel(w http.ResponseWriter, r *http.Request, admin *store.User) {
	var in modelInput
	if err := httpx.DecodeJSON(r, 64<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	m, msg := in.toModel()
	if msg != "" {
		httpx.Error(w, 400, msg)
		return
	}
	m.IsCustom = true
	created, err := s.St.CreateModel(r.Context(), *m)
	if err != nil {
		s.fail(w, err)
		return
	}
	if created.PricingMode == "auto" {
		_, _ = s.St.RepriceAuto(r.Context(), s.Catalog.Snapshot().Settings.DefaultMarkupBps)
		created, _ = s.St.GetModel(r.Context(), created.ID)
	}
	s.catalogChanged(r)
	s.audit(r, admin, "model.create", created.Slug, map[string]any{"upstream": created.UpstreamID})
	httpx.JSON(w, 201, created)
}

func (s *Server) updateModel(w http.ResponseWriter, r *http.Request, admin *store.User) {
	id, ok := pathInt(r, "id")
	if !ok {
		httpx.Error(w, 400, "invalid id")
		return
	}
	existing, err := s.St.GetModel(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	var in modelInput
	if err := httpx.DecodeJSON(r, 64<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	m, msg := in.toModel()
	if msg != "" {
		httpx.Error(w, 400, msg)
		return
	}
	m.ID = id
	m.IsCustom = existing.IsCustom
	updated, err := s.St.UpdateModel(r.Context(), *m)
	if err != nil {
		s.fail(w, err)
		return
	}
	if updated.PricingMode == "auto" {
		_, _ = s.St.RepriceAuto(r.Context(), s.Catalog.Snapshot().Settings.DefaultMarkupBps)
		updated, _ = s.St.GetModel(r.Context(), id)
	}
	s.catalogChanged(r)
	s.audit(r, admin, "model.update", updated.Slug, nil)
	httpx.JSON(w, 200, updated)
}

func (s *Server) deleteModel(w http.ResponseWriter, r *http.Request, admin *store.User) {
	id, ok := pathInt(r, "id")
	if !ok {
		httpx.Error(w, 400, "invalid id")
		return
	}
	if err := s.St.DeleteModel(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	s.catalogChanged(r)
	s.audit(r, admin, "model.delete", strconv.FormatInt(id, 10), nil)
	httpx.JSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) bulkModels(w http.ResponseWriter, r *http.Request, admin *store.User) {
	var in struct {
		IDs     []int64 `json:"ids"`
		Enabled bool    `json:"enabled"`
	}
	if err := httpx.DecodeJSON(r, 1<<20, &in); err != nil || len(in.IDs) == 0 {
		httpx.Error(w, 400, "ids required")
		return
	}
	n, err := s.St.SetModelsEnabled(r.Context(), in.IDs, in.Enabled)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.catalogChanged(r)
	s.audit(r, admin, "model.bulk_enable", "", map[string]any{"count": n, "enabled": in.Enabled})
	httpx.JSON(w, 200, map[string]int64{"updated": n})
}

func (s *Server) reprice(w http.ResponseWriter, r *http.Request, admin *store.User) {
	settings := s.Catalog.Snapshot().Settings
	n, err := s.St.RepriceAuto(r.Context(), settings.DefaultMarkupBps)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.catalogChanged(r)
	s.audit(r, admin, "model.reprice", "", map[string]any{"count": n})
	httpx.JSON(w, 200, map[string]int64{"repriced": n})
}

func (s *Server) listUpstream(w http.ResponseWriter, r *http.Request, _ *store.User) {
	pid, _ := strconv.ParseInt(r.URL.Query().Get("provider_id"), 10, 64)
	ms, total, err := s.St.ListUpstreamModels(r.Context(), pid, r.URL.Query().Get("q"),
		httpx.QueryInt(r, "limit", 50, 1, 500), httpx.QueryInt(r, "offset", 0, 0, 1_000_000))
	if err != nil {
		s.fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"data": ms, "total": total})
}

func (s *Server) publish(w http.ResponseWriter, r *http.Request, admin *store.User) {
	var in struct {
		ProviderID  int64    `json:"provider_id"`
		UpstreamIDs []string `json:"upstream_ids"`
		MarkupBps   *int     `json:"markup_bps"`
		Enabled     *bool    `json:"enabled"`
	}
	if err := httpx.DecodeJSON(r, 4<<20, &in); err != nil || in.ProviderID == 0 || len(in.UpstreamIDs) == 0 {
		httpx.Error(w, 400, "provider_id and upstream_ids required")
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	n, err := s.St.PublishUpstream(r.Context(), in.ProviderID, in.UpstreamIDs, in.MarkupBps, enabled)
	if err != nil {
		s.fail(w, err)
		return
	}
	if _, err := s.St.RepriceAuto(r.Context(), s.Catalog.Snapshot().Settings.DefaultMarkupBps); err != nil {
		s.fail(w, err)
		return
	}
	s.catalogChanged(r)
	s.audit(r, admin, "model.publish", "", map[string]any{"count": n, "provider_id": in.ProviderID})
	httpx.JSON(w, 200, map[string]int64{"published": n})
}

// ---------------------------------------------------------------------------
// Providers
// ---------------------------------------------------------------------------

func (s *Server) listProviders(w http.ResponseWriter, r *http.Request, _ *store.User) {
	ps, err := s.St.ListProviders(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"data": ps})
}

type providerInput struct {
	Name         *string           `json:"name"`
	Kind         *string           `json:"kind"`
	BaseURL      *string           `json:"base_url"`
	APIKey       *string           `json:"api_key"`
	Headers      map[string]string `json:"headers"`
	IsOpenRouter *bool             `json:"is_openrouter"`
	Enabled      *bool             `json:"enabled"`
}

func (in *providerInput) validate() string {
	if in.Kind != nil && *in.Kind != upstream.KindOpenAI && *in.Kind != upstream.KindAnthropic {
		return "kind must be openai or anthropic"
	}
	if in.BaseURL != nil && !strings.HasPrefix(*in.BaseURL, "https://") && !strings.HasPrefix(*in.BaseURL, "http://") {
		return "base_url must be an http(s) URL"
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		return "name is required"
	}
	return ""
}

func (s *Server) createProvider(w http.ResponseWriter, r *http.Request, admin *store.User) {
	var in providerInput
	if err := httpx.DecodeJSON(r, 64<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	if in.Name == nil || in.Kind == nil || in.BaseURL == nil {
		httpx.Error(w, 400, "name, kind and base_url are required")
		return
	}
	if msg := in.validate(); msg != "" {
		httpx.Error(w, 400, msg)
		return
	}
	p := store.Provider{Name: strings.TrimSpace(*in.Name), Kind: *in.Kind, BaseURL: *in.BaseURL, Headers: in.Headers, Enabled: true}
	if in.Enabled != nil {
		p.Enabled = *in.Enabled
	}
	if in.IsOpenRouter != nil {
		p.IsOpenRouter = *in.IsOpenRouter
	}
	if in.APIKey != nil {
		enc, err := s.Box.Encrypt(*in.APIKey)
		if err != nil {
			httpx.Error(w, 500, "encryption failed")
			return
		}
		p.APIKeyEnc = enc
	}
	created, err := s.St.CreateProvider(r.Context(), p)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.catalogChanged(r)
	s.audit(r, admin, "provider.create", created.Name, nil)
	httpx.JSON(w, 201, created)
}

func (s *Server) updateProvider(w http.ResponseWriter, r *http.Request, admin *store.User) {
	id, ok := pathInt(r, "id")
	if !ok {
		httpx.Error(w, 400, "invalid id")
		return
	}
	var in providerInput
	if err := httpx.DecodeJSON(r, 64<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	if msg := in.validate(); msg != "" {
		httpx.Error(w, 400, msg)
		return
	}
	patch := store.ProviderPatch{Name: in.Name, Kind: in.Kind, BaseURL: in.BaseURL, Headers: in.Headers,
		IsOpenRouter: in.IsOpenRouter, Enabled: in.Enabled}
	if in.APIKey != nil && *in.APIKey != "" {
		enc, err := s.Box.Encrypt(*in.APIKey)
		if err != nil {
			httpx.Error(w, 500, "encryption failed")
			return
		}
		patch.APIKeyEnc = &enc
	}
	p, err := s.St.UpdateProvider(r.Context(), id, patch)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.catalogChanged(r)
	s.audit(r, admin, "provider.update", p.Name, map[string]any{"key_changed": patch.APIKeyEnc != nil})
	httpx.JSON(w, 200, p)
}

func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request, admin *store.User) {
	id, ok := pathInt(r, "id")
	if !ok {
		httpx.Error(w, 400, "invalid id")
		return
	}
	if err := s.St.DeleteProvider(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrConflict) {
			httpx.Error(w, 409, "provider still has models; delete or move them first")
			return
		}
		s.fail(w, err)
		return
	}
	s.catalogChanged(r)
	s.audit(r, admin, "provider.delete", strconv.FormatInt(id, 10), nil)
	httpx.JSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) syncProvider(w http.ResponseWriter, r *http.Request, admin *store.User) {
	id, ok := pathInt(r, "id")
	if !ok {
		httpx.Error(w, 400, "invalid id")
		return
	}
	res, err := s.Catalog.SyncProvider(r.Context(), id)
	if err != nil {
		s.Log.Warn("manual sync failed", "provider", id, "err", err)
		httpx.Error(w, 502, "sync failed: "+err.Error())
		return
	}
	s.Bus.Publish(r.Context(), bus.KindCatalog, "")
	s.audit(r, admin, "provider.sync", strconv.FormatInt(id, 10), map[string]any{"fetched": res.Fetched, "added": len(res.Added)})
	httpx.JSON(w, 200, res)
}

// ---------------------------------------------------------------------------
// Payments, usage, settings, audit
// ---------------------------------------------------------------------------

func (s *Server) listPayments(w http.ResponseWriter, r *http.Request, _ *store.User) {
	ps, total, err := s.St.ListPayments(r.Context(), r.URL.Query().Get("user_id"), r.URL.Query().Get("status"),
		httpx.QueryInt(r, "limit", 50, 1, 200), httpx.QueryInt(r, "offset", 0, 0, 10_000_000))
	if err != nil {
		s.fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"data": ps, "total": total})
}

func (s *Server) listUsage(w http.ResponseWriter, r *http.Request, _ *store.User) {
	q := r.URL.Query()
	before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
	recs, err := s.St.ListUsage(r.Context(), store.UsageFilter{
		UserID: q.Get("user_id"), KeyID: q.Get("key_id"), Model: q.Get("model"), BeforeID: before,
		Limit: httpx.QueryInt(r, "limit", 100, 1, 200), ErrorsOnly: q.Get("errors") == "1",
	})
	if err != nil {
		s.fail(w, err)
		return
	}
	var next any
	if len(recs) > 0 {
		next = recs[len(recs)-1].ID
	}
	httpx.JSON(w, 200, map[string]any{"data": recs, "next_before": next})
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request, _ *store.User) {
	st, err := s.St.GetSettings(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	httpx.JSON(w, 200, st)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request, admin *store.User) {
	st, err := s.St.GetSettings(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := httpx.DecodeJSON(r, 64<<10, &st); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	switch {
	case st.DefaultMarkupBps < -10000 || st.DefaultMarkupBps > 1_000_000:
		httpx.Error(w, 400, "markup out of range")
		return
	case st.DefaultRequestFee < 0 || st.SignupBonus < 0:
		httpx.Error(w, 400, "fees and bonuses must be non-negative")
		return
	case st.MinTopupCents < 100 || st.MaxTopupCents < st.MinTopupCents:
		httpx.Error(w, 400, "invalid top-up range (minimum is $1)")
		return
	case st.BDTPerUSD <= 0:
		httpx.Error(w, 400, "BDT rate must be positive")
		return
	case st.DefaultRPM < 0 || st.FreeModelRPM < 0 || st.DefaultMaxOutputTokens < 0:
		httpx.Error(w, 400, "limits must be non-negative")
		return
	}
	if err := s.St.PutSettings(r.Context(), st); err != nil {
		s.fail(w, err)
		return
	}
	if _, err := s.St.RepriceAuto(r.Context(), st.DefaultMarkupBps); err != nil {
		s.fail(w, err)
		return
	}
	s.catalogChanged(r)
	s.audit(r, admin, "settings.update", "", map[string]any{"settings": st})
	httpx.JSON(w, 200, st)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request, _ *store.User) {
	logs, err := s.St.ListAudit(r.Context(), httpx.QueryInt(r, "limit", 100, 1, 500), httpx.QueryInt(r, "offset", 0, 0, 10_000_000))
	if err != nil {
		s.fail(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"data": logs})
}
