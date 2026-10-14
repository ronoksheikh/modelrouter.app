// Package paddle implements international card/PayPal top-ups via Paddle Billing.
//
// Flow: the backend creates a Paddle transaction with a non-catalog price for
// the exact top-up amount and our payment id in custom_data. The dashboard
// opens the Paddle.js overlay for that transaction (or redirects to the hosted
// checkout URL). Paddle then calls our webhook with `transaction.completed`,
// which credits the user exactly once.
package paddle

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/payments"
)

type Provider struct {
	APIKey        string
	WebhookSecret string
	Environment   string
	ProductID     string
	HTTP          *http.Client
	Now           func() time.Time
	baseURL       string
}

func New(apiKey, webhookSecret, env, productID string) *Provider {
	base := "https://sandbox-api.paddle.com"
	if env == "production" {
		base = "https://api.paddle.com"
	}
	return &Provider{APIKey: apiKey, WebhookSecret: webhookSecret, Environment: env, ProductID: productID,
		HTTP: &http.Client{Timeout: 20 * time.Second}, Now: time.Now, baseURL: base}
}

// WithBaseURL overrides the API endpoint (tests).
func (p *Provider) WithBaseURL(u string) *Provider { p.baseURL = u; return p }

func (p *Provider) ID() string { return "paddle" }

func (p *Provider) Method() payments.Method {
	return payments.Method{
		ID: "paddle", Name: "Card or PayPal", Description: "Visa, Mastercard, Amex, Apple Pay, Google Pay and PayPal. Processed securely by Paddle.",
		Currency: "USD", Regions: []string{payments.RegionGlobal}, Logos: []string{"visa", "mastercard", "amex", "paypal"},
	}
}

type txItem struct {
	Quantity int     `json:"quantity"`
	Price    txPrice `json:"price"`
}

type txPrice struct {
	Description string         `json:"description"`
	Name        string         `json:"name"`
	ProductID   string         `json:"product_id,omitempty"`
	Product     map[string]any `json:"product,omitempty"`
	UnitPrice   map[string]any `json:"unit_price"`
	TaxMode     string         `json:"tax_mode"`
	Quantity    map[string]int `json:"quantity"`
}

func (p *Provider) CreateCheckout(ctx context.Context, req payments.CheckoutRequest) (*payments.Checkout, error) {
	price := txPrice{
		Description: req.Description,
		Name:        "ModelRouter credits",
		UnitPrice:   map[string]any{"amount": strconv.FormatInt(req.AmountMinor, 10), "currency_code": req.Currency},
		TaxMode:     "account_setting",
		Quantity:    map[string]int{"minimum": 1, "maximum": 1},
	}
	if p.ProductID != "" {
		price.ProductID = p.ProductID
	} else {
		price.Product = map[string]any{"name": "ModelRouter credits", "tax_category": "standard"}
	}
	payload := map[string]any{
		"items":           []txItem{{Quantity: 1, Price: price}},
		"custom_data":     map[string]string{"payment_id": req.PaymentID, "user_id": req.UserID},
		"collection_mode": "automatic",
	}
	if req.SuccessURL != "" {
		payload["checkout"] = map[string]string{"url": req.SuccessURL}
	}
	body, _ := json.Marshal(payload)
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/transactions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Authorization", "Bearer "+p.APIKey)
	hreq.Header.Set("Content-Type", "application/json")
	resp, err := p.HTTP.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("paddle: create transaction: status %d: %s", resp.StatusCode, strings.TrimSpace(string(rb)))
	}
	var out struct {
		Data struct {
			ID       string `json:"id"`
			Checkout *struct {
				URL string `json:"url"`
			} `json:"checkout"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rb, &out); err != nil {
		return nil, fmt.Errorf("paddle: decode transaction: %w", err)
	}
	co := &payments.Checkout{
		ExternalID: out.Data.ID,
		ClientData: map[string]any{"provider": "paddle", "transaction_id": out.Data.ID, "environment": p.Environment},
	}
	if out.Data.Checkout != nil {
		co.RedirectURL = out.Data.Checkout.URL
	}
	return co, nil
}

// VerifySignature checks a `Paddle-Signature: ts=...;h1=...` header.
func VerifySignature(secret, header string, body []byte, now time.Time, tolerance time.Duration) error {
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "ts":
			ts = v
		case "h1":
			sigs = append(sigs, v)
		}
	}
	if ts == "" || len(sigs) == 0 {
		return payments.ErrBadSignature
	}
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return payments.ErrBadSignature
	}
	if d := now.Sub(time.Unix(t, 0)); d > tolerance || d < -tolerance {
		return errors.New("webhook timestamp outside tolerance")
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(ts + ":"))
	m.Write(body)
	expected := m.Sum(nil)
	for _, s := range sigs {
		got, err := hex.DecodeString(s)
		if err == nil && hmac.Equal(got, expected) {
			return nil
		}
	}
	return payments.ErrBadSignature
}

func (p *Provider) ParseWebhook(_ context.Context, r *http.Request, body []byte) (*payments.WebhookEvent, error) {
	if err := VerifySignature(p.WebhookSecret, r.Header.Get("Paddle-Signature"), body, p.Now(), 5*time.Minute); err != nil {
		return nil, err
	}
	var n struct {
		EventType string `json:"event_type"`
		Data      struct {
			ID         string            `json:"id"`
			Status     string            `json:"status"`
			Currency   string            `json:"currency_code"`
			CustomData map[string]string `json:"custom_data"`
			Details    struct {
				Totals struct {
					GrandTotal string `json:"grand_total"`
					Subtotal   string `json:"subtotal"`
				} `json:"totals"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, fmt.Errorf("paddle: decode webhook: %w", err)
	}
	ev := &payments.WebhookEvent{
		PaymentID:  n.Data.CustomData["payment_id"],
		ExternalID: n.Data.ID,
		Currency:   n.Data.Currency,
		Raw:        map[string]any{"event_type": n.EventType, "status": n.Data.Status},
	}
	// Subtotal excludes tax, which is what the customer pays for credits.
	ev.AmountMinor, _ = strconv.ParseInt(n.Data.Details.Totals.Subtotal, 10, 64)
	switch n.EventType {
	case "transaction.completed", "transaction.paid":
		ev.Status = "completed"
	case "transaction.payment_failed":
		ev.Status = "failed"
	case "transaction.canceled":
		ev.Status = "canceled"
	default:
		ev.Status = "ignore"
	}
	return ev, nil
}
