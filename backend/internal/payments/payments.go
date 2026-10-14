// Package payments defines the pluggable top-up provider interface.
//
// Adding a payment method (e.g. bKash, Nagad, SSLCommerz for Bangladesh) is a
// code change: implement Provider in a sub-package and register it in
// server.buildPayments. Each provider declares the regions it serves, so the
// dashboard automatically shows Bangladeshi methods to BD users and
// international methods (Paddle) to everyone else.
package payments

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
)

// RegionBD is the ISO country code of Bangladesh.
const RegionBD = "BD"

// RegionGlobal matches every country.
const RegionGlobal = "*"

// Checkout is what the dashboard needs to send the user to pay.
type Checkout struct {
	// RedirectURL sends the browser to a hosted payment page.
	RedirectURL string `json:"redirect_url,omitempty"`
	// ClientData is provider-specific data for an embedded checkout
	// (e.g. Paddle.js transaction id).
	ClientData map[string]any `json:"client_data,omitempty"`
	// ExternalID is the provider's reference for this checkout.
	ExternalID string `json:"-"`
}

type CheckoutRequest struct {
	PaymentID   string // our payments.id, must be echoed back by the provider
	UserID      string
	Email       string
	Name        string
	Currency    string
	AmountMinor int64 // amount in the currency's minor unit (cents, poisha)
	Description string
	SuccessURL  string
	CancelURL   string
	WebhookURL  string
}

// WebhookEvent is a normalized provider notification.
type WebhookEvent struct {
	PaymentID   string // our payments.id
	ExternalID  string
	Status      string // completed | failed | canceled | refunded | ignore
	Currency    string
	AmountMinor int64
	Raw         map[string]any
}

// Method is the public description shown in the dashboard.
type Method struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Currency    string   `json:"currency"`
	Regions     []string `json:"regions"`
	Logos       []string `json:"logos"`
	Available   bool     `json:"available"`
}

type Provider interface {
	// ID is a stable identifier stored on payments rows, e.g. "paddle".
	ID() string
	Method() Method
	CreateCheckout(ctx context.Context, req CheckoutRequest) (*Checkout, error)
	// ParseWebhook authenticates and parses a provider notification.
	ParseWebhook(ctx context.Context, r *http.Request, body []byte) (*WebhookEvent, error)
}

var ErrUnknownProvider = errors.New("unknown payment provider")
var ErrBadSignature = errors.New("invalid webhook signature")

type Registry struct {
	providers map[string]Provider
	// upcoming lists methods that are not integrated yet; shown as "coming soon".
	upcoming []Method
}

func NewRegistry() *Registry { return &Registry{providers: map[string]Provider{}} }

func (r *Registry) Register(p Provider) { r.providers[p.ID()] = p }

// AddUpcoming lists a not-yet-integrated method in the UI as "coming soon".
func (r *Registry) AddUpcoming(m Method) {
	m.Available = false
	r.upcoming = append(r.upcoming, m)
}

func (r *Registry) Get(id string) (Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

// MethodsFor returns the methods to show to a user from the given country.
// Bangladeshi users see local methods first, then international ones.
func (r *Registry) MethodsFor(country string) []Method {
	country = strings.ToUpper(country)
	var out []Method
	add := func(m Method) {
		for _, reg := range m.Regions {
			if reg == RegionGlobal || reg == country {
				out = append(out, m)
				return
			}
		}
	}
	for _, p := range r.providers {
		m := p.Method()
		m.Available = true
		add(m)
	}
	for _, m := range r.upcoming {
		add(m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		li, lj := isLocal(out[i], country), isLocal(out[j], country)
		if li != lj {
			return li
		}
		if out[i].Available != out[j].Available {
			return out[i].Available
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func isLocal(m Method, country string) bool {
	for _, r := range m.Regions {
		if r == country {
			return true
		}
	}
	return false
}

// AllowedFor reports whether provider id may be used by a user from country.
func (r *Registry) AllowedFor(id, country string) bool {
	p, ok := r.providers[id]
	if !ok {
		return false
	}
	for _, reg := range p.Method().Regions {
		if reg == RegionGlobal || strings.EqualFold(reg, country) {
			return true
		}
	}
	return false
}
