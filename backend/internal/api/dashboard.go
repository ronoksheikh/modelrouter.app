package api

import (
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/billing"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/bus"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/httpx"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/payments"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/secret"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
)

func (s *Server) me(w http.ResponseWriter, r *http.Request, u *store.User) {
	avail, err := s.Ledger.Available(r.Context(), u.ID)
	if err != nil {
		avail = u.Balance
	}
	httpx.JSON(w, 200, map[string]any{"user": u, "available_balance": avail})
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request, u *store.User) {
	var in struct {
		Name    *string `json:"name"`
		Country *string `json:"country"`
	}
	if err := httpx.DecodeJSON(r, 16<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	if in.Name != nil && utf8.RuneCountInString(*in.Name) > 80 {
		httpx.Error(w, 400, "name too long")
		return
	}
	if in.Country != nil {
		c := strings.ToUpper(strings.TrimSpace(*in.Country))
		if len(c) != 2 && c != "" {
			httpx.Error(w, 400, "country must be an ISO 3166-1 alpha-2 code")
			return
		}
		in.Country = &c
	}
	nu, err := s.St.UpdateUser(r.Context(), u.ID, store.UserPatch{Name: in.Name, Country: in.Country})
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	s.Sess.Invalidate(u.ID)
	httpx.JSON(w, 200, map[string]any{"user": nu})
}

// ---------------------------------------------------------------------------
// API keys
// ---------------------------------------------------------------------------

type keyInput struct {
	Name          *string  `json:"name"`
	Disabled      *bool    `json:"disabled"`
	SpendLimitUSD *float64 `json:"spend_limit_usd"` // null clears when clear_limit=true
	ClearLimit    bool     `json:"clear_limit"`
	RPMLimit      *int     `json:"rpm_limit"`
}

func (in *keyInput) validate() string {
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || utf8.RuneCountInString(n) > 64 {
			return "name must be 1-64 characters"
		}
		in.Name = &n
	}
	if in.SpendLimitUSD != nil && (*in.SpendLimitUSD < 0 || *in.SpendLimitUSD > 1e6 || math.IsNaN(*in.SpendLimitUSD)) {
		return "spend limit must be between 0 and 1,000,000 USD"
	}
	if in.RPMLimit != nil && (*in.RPMLimit < 1 || *in.RPMLimit > 100_000) {
		return "rpm limit must be between 1 and 100000"
	}
	return ""
}

func usdToNano(v *float64) *int64 {
	if v == nil {
		return nil
	}
	n := int64(math.Round(*v * float64(billing.NanoPerUSD)))
	return &n
}

func (s *Server) listKeys(w http.ResponseWriter, r *http.Request, u *store.User) {
	keys, err := s.St.ListKeys(r.Context(), u.ID)
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"data": keys})
}

func (s *Server) createKey(w http.ResponseWriter, r *http.Request, u *store.User) {
	var in keyInput
	if err := httpx.DecodeJSON(r, 16<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	if in.Name == nil {
		n := "Default"
		in.Name = &n
	}
	if msg := in.validate(); msg != "" {
		httpx.Error(w, 400, msg)
		return
	}
	n, err := s.St.CountKeys(r.Context(), u.ID)
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	if n >= maxKeysPerUser {
		httpx.Error(w, 400, "key limit reached; delete an unused key first")
		return
	}
	raw, err := secret.NewAPIKey()
	if err != nil {
		httpx.Error(w, 500, "could not generate key")
		return
	}
	k, err := s.St.CreateKey(r.Context(), u.ID, *in.Name, secret.HashKey(raw), secret.DisplayPrefix(raw), usdToNano(in.SpendLimitUSD), in.RPMLimit)
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	httpx.JSON(w, 201, map[string]any{"key": k, "secret": raw})
}

func (s *Server) updateKey(w http.ResponseWriter, r *http.Request, u *store.User) {
	var in keyInput
	if err := httpx.DecodeJSON(r, 16<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	if msg := in.validate(); msg != "" {
		httpx.Error(w, 400, msg)
		return
	}
	k, hash, err := s.St.UpdateKey(r.Context(), u.ID, r.PathValue("id"), store.KeyPatch{
		Name: in.Name, Disabled: in.Disabled, SpendLimit: usdToNano(in.SpendLimitUSD), ClearLimit: in.ClearLimit, RPMLimit: in.RPMLimit,
	})
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	s.Bus.Publish(r.Context(), bus.KindKey, hash)
	httpx.JSON(w, 200, map[string]any{"key": k})
}

func (s *Server) rotateKey(w http.ResponseWriter, r *http.Request, u *store.User) {
	raw, err := secret.NewAPIKey()
	if err != nil {
		httpx.Error(w, 500, "could not generate key")
		return
	}
	k, old, err := s.St.RotateKey(r.Context(), u.ID, r.PathValue("id"), secret.HashKey(raw), secret.DisplayPrefix(raw))
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	s.Bus.Publish(r.Context(), bus.KindKey, old)
	httpx.JSON(w, 200, map[string]any{"key": k, "secret": raw})
}

func (s *Server) deleteKey(w http.ResponseWriter, r *http.Request, u *store.User) {
	hash, err := s.St.DeleteKey(r.Context(), u.ID, r.PathValue("id"))
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	s.Bus.Publish(r.Context(), bus.KindKey, hash)
	httpx.JSON(w, 200, map[string]bool{"ok": true})
}

// ---------------------------------------------------------------------------
// Usage
// ---------------------------------------------------------------------------

func (s *Server) usageDaily(w http.ResponseWriter, r *http.Request, u *store.User) {
	days := httpx.QueryInt(r, "days", 30, 1, 365)
	d, err := s.St.DailyUsage(r.Context(), u.ID, days)
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	for i := range d {
		d[i].UpstreamCost = 0
	}
	httpx.JSON(w, 200, map[string]any{"data": d})
}

func (s *Server) usageModels(w http.ResponseWriter, r *http.Request, u *store.User) {
	days := httpx.QueryInt(r, "days", 30, 1, 365)
	d, err := s.St.ModelUsage(r.Context(), u.ID, days, 20)
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	for i := range d {
		d[i].UpstreamCost = 0
	}
	httpx.JSON(w, 200, map[string]any{"data": d})
}

func (s *Server) activity(w http.ResponseWriter, r *http.Request, u *store.User) {
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	recs, err := s.St.ListUsage(r.Context(), store.UsageFilter{
		UserID: u.ID, KeyID: r.URL.Query().Get("key"), Model: r.URL.Query().Get("model"),
		BeforeID: before, Limit: httpx.QueryInt(r, "limit", 50, 1, 200),
	})
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	// Never expose upstream routing or cost to customers.
	for i := range recs {
		recs[i].UpstreamID, recs[i].ProviderID, recs[i].UpstreamCost = "", 0, 0
	}
	var next any
	if len(recs) > 0 {
		next = recs[len(recs)-1].ID
	}
	httpx.JSON(w, 200, map[string]any{"data": recs, "next_before": next})
}

// ---------------------------------------------------------------------------
// Billing
// ---------------------------------------------------------------------------

func (s *Server) country(r *http.Request, u *store.User) string {
	if c := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("country"))); len(c) == 2 {
		return c
	}
	return u.Country
}

func (s *Server) billingMethods(w http.ResponseWriter, r *http.Request, u *store.User) {
	st := s.Catalog.Snapshot().Settings
	httpx.JSON(w, 200, map[string]any{
		"methods":         s.Payments.MethodsFor(s.country(r, u)),
		"min_topup_cents": st.MinTopupCents,
		"max_topup_cents": st.MaxTopupCents,
		"bdt_per_usd":     st.BDTPerUSD,
	})
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request, u *store.User) {
	var in struct {
		Method      string `json:"method"`
		AmountCents int64  `json:"amount_cents"`
		Country     string `json:"country"`
	}
	if err := httpx.DecodeJSON(r, 16<<10, &in); err != nil {
		httpx.Error(w, 400, "invalid body")
		return
	}
	st := s.Catalog.Snapshot().Settings
	if in.AmountCents < st.MinTopupCents || in.AmountCents > st.MaxTopupCents {
		httpx.Error(w, 400, "amount must be between $"+billing.FormatUSD(st.MinTopupCents*billing.NanoPerCent)+
			" and $"+billing.FormatUSD(st.MaxTopupCents*billing.NanoPerCent))
		return
	}
	country := strings.ToUpper(in.Country)
	if len(country) != 2 {
		country = u.Country
	}
	p, ok := s.Payments.Get(in.Method)
	if !ok || !s.Payments.AllowedFor(in.Method, country) {
		httpx.Error(w, 400, "this payment method is not available")
		return
	}
	method := p.Method()
	amountMinor := in.AmountCents
	if method.Currency == "BDT" {
		amountMinor = int64(math.Ceil(float64(in.AmountCents) * st.BDTPerUSD))
	}
	credit := in.AmountCents * billing.NanoPerCent
	pay, err := s.St.CreatePayment(r.Context(), u.ID, p.ID(), method.Currency, amountMinor, credit,
		map[string]any{"amount_usd_cents": in.AmountCents, "country": country})
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	web := s.Cfg.WebURL
	co, err := p.CreateCheckout(r.Context(), payments.CheckoutRequest{
		PaymentID: pay.ID, UserID: u.ID, Email: u.Email, Name: u.Name, Currency: method.Currency, AmountMinor: amountMinor,
		Description: "$" + billing.FormatUSD(credit) + " ModelRouter credits",
		SuccessURL:  web + "/dashboard/billing?payment=" + pay.ID,
		CancelURL:   web + "/dashboard/billing?payment=" + pay.ID + "&canceled=1",
		WebhookURL:  s.Cfg.PublicURL + "/webhooks/" + p.ID(),
	})
	if err != nil {
		s.Log.Error("create checkout failed", "provider", p.ID(), "err", err)
		_ = s.St.SetPaymentStatus(r.Context(), pay.ID, "failed")
		httpx.Error(w, 502, "could not start checkout, please try again")
		return
	}
	if co.ExternalID != "" {
		_ = s.St.SetPaymentExternal(r.Context(), pay.ID, co.ExternalID, nil)
	}
	httpx.JSON(w, 200, map[string]any{"payment_id": pay.ID, "checkout": co})
}

func (s *Server) listPayments(w http.ResponseWriter, r *http.Request, u *store.User) {
	ps, total, err := s.St.ListPayments(r.Context(), u.ID, "", httpx.QueryInt(r, "limit", 20, 1, 100), httpx.QueryInt(r, "offset", 0, 0, 1_000_000))
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"data": ps, "total": total})
}

func (s *Server) getPayment(w http.ResponseWriter, r *http.Request, u *store.User) {
	p, err := s.St.GetPayment(r.Context(), r.PathValue("id"))
	if err != nil || p.UserID != u.ID {
		httpx.Error(w, 404, "not found")
		return
	}
	httpx.JSON(w, 200, p)
}

func (s *Server) ledger(w http.ResponseWriter, r *http.Request, u *store.User) {
	es, err := s.St.ListLedger(r.Context(), u.ID, httpx.QueryInt(r, "limit", 50, 1, 200))
	if err != nil {
		storeErr(w, s.Log, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"data": es})
}

// webhook receives provider notifications and credits completed payments.
func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	p, ok := s.Payments.Get(r.PathValue("provider"))
	if !ok {
		httpx.Error(w, 404, "unknown provider")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.Error(w, 400, "bad body")
		return
	}
	ev, err := p.ParseWebhook(r.Context(), r, body)
	if err != nil {
		s.Log.Warn("webhook rejected", "provider", p.ID(), "err", err)
		httpx.Error(w, 400, "invalid webhook")
		return
	}
	if ev.PaymentID == "" || ev.Status == "ignore" {
		httpx.JSON(w, 200, map[string]bool{"ok": true})
		return
	}
	pay, err := s.St.GetPayment(r.Context(), ev.PaymentID)
	if errors.Is(err, store.ErrNotFound) {
		s.Log.Warn("webhook for unknown payment", "provider", p.ID(), "payment", ev.PaymentID)
		httpx.JSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if err != nil {
		httpx.Error(w, 500, "temporary error") // provider will retry
		return
	}
	if pay.Provider != p.ID() {
		httpx.Error(w, 400, "provider mismatch")
		return
	}
	switch ev.Status {
	case "completed":
		if ev.Currency != "" && !strings.EqualFold(ev.Currency, pay.Currency) || ev.AmountMinor > 0 && ev.AmountMinor < pay.AmountMinor {
			s.Log.Error("payment amount mismatch; not crediting", "payment", pay.ID, "expected", pay.AmountMinor,
				"got", ev.AmountMinor, "currency", ev.Currency)
			httpx.JSON(w, 200, map[string]bool{"ok": true})
			return
		}
		done, err := s.St.CompletePayment(r.Context(), pay.ID, ev.ExternalID, ev.Raw)
		if errors.Is(err, store.ErrAlreadyProcessed) {
			httpx.JSON(w, 200, map[string]bool{"ok": true})
			return
		}
		if err != nil {
			s.Log.Error("complete payment failed", "payment", pay.ID, "err", err)
			httpx.Error(w, 500, "temporary error")
			return
		}
		s.Bus.Publish(r.Context(), bus.KindUser, done.UserID)
		s.Log.Info("payment completed", "payment", done.ID, "provider", p.ID(), "credit", billing.FormatUSD(done.Credit))
	case "failed", "canceled":
		_ = s.St.SetPaymentStatus(r.Context(), pay.ID, ev.Status)
	}
	httpx.JSON(w, 200, map[string]bool{"ok": true})
}
