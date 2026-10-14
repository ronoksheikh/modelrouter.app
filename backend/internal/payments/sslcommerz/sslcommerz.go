// Package sslcommerz is a Bangladeshi payment gateway (bKash, Nagad, Rocket,
// local cards, internet banking) behind one hosted checkout.
//
// It is NOT registered unless SSLCOMMERZ_STORE_ID / SSLCOMMERZ_STORE_PASSWORD
// are set, and it doubles as the reference implementation for adding any other
// Bangladeshi method (bKash/Nagad direct APIs follow the same shape:
// create session → redirect → server-to-server notification → validation).
package sslcommerz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/payments"
)

type Provider struct {
	StoreID     string
	StorePasswd string
	HTTP        *http.Client
	baseURL     string
}

func New(storeID, passwd string, sandbox bool) *Provider {
	base := "https://securepay.sslcommerz.com"
	if sandbox {
		base = "https://sandbox.sslcommerz.com"
	}
	return &Provider{StoreID: storeID, StorePasswd: passwd, HTTP: &http.Client{Timeout: 20 * time.Second}, baseURL: base}
}

func (p *Provider) WithBaseURL(u string) *Provider { p.baseURL = u; return p }

func (p *Provider) ID() string { return "sslcommerz" }

func (p *Provider) Method() payments.Method {
	return payments.Method{
		ID: "sslcommerz", Name: "bKash, Nagad & local cards", Description: "Pay in BDT with bKash, Nagad, Rocket, local cards or internet banking.",
		Currency: "BDT", Regions: []string{payments.RegionBD}, Logos: []string{"bkash", "nagad", "rocket"},
	}
}

func (p *Provider) CreateCheckout(ctx context.Context, req payments.CheckoutRequest) (*payments.Checkout, error) {
	form := url.Values{
		"store_id":         {p.StoreID},
		"store_passwd":     {p.StorePasswd},
		"total_amount":     {minorToDecimal(req.AmountMinor)},
		"currency":         {req.Currency},
		"tran_id":          {req.PaymentID},
		"success_url":      {req.SuccessURL},
		"fail_url":         {req.CancelURL},
		"cancel_url":       {req.CancelURL},
		"ipn_url":          {req.WebhookURL},
		"cus_name":         {nonEmpty(req.Name, "Customer")},
		"cus_email":        {req.Email},
		"cus_add1":         {"N/A"},
		"cus_city":         {"Dhaka"},
		"cus_country":      {"Bangladesh"},
		"cus_phone":        {"N/A"},
		"shipping_method":  {"NO"},
		"product_name":     {"ModelRouter credits"},
		"product_category": {"Digital"},
		"product_profile":  {"non-physical-goods"},
		"value_a":          {req.UserID},
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/gwprocess/v4/api.php", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.HTTP.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Status         string `json:"status"`
		FailedReason   string `json:"failedreason"`
		SessionKey     string `json:"sessionkey"`
		GatewayPageURL string `json:"GatewayPageURL"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("sslcommerz: decode session: %w", err)
	}
	if out.Status != "SUCCESS" || out.GatewayPageURL == "" {
		return nil, fmt.Errorf("sslcommerz: create session failed: %s", out.FailedReason)
	}
	return &payments.Checkout{RedirectURL: out.GatewayPageURL, ExternalID: out.SessionKey}, nil
}

// ParseWebhook handles the IPN. The IPN body itself is not trusted: the
// transaction is re-validated server-to-server with the validator API.
func (p *Provider) ParseWebhook(ctx context.Context, _ *http.Request, body []byte) (*payments.WebhookEvent, error) {
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	tranID := form.Get("tran_id")
	switch form.Get("status") {
	case "FAILED":
		return &payments.WebhookEvent{PaymentID: tranID, Status: "failed"}, nil
	case "CANCELLED":
		return &payments.WebhookEvent{PaymentID: tranID, Status: "canceled"}, nil
	case "VALID", "VALIDATED":
	default:
		return &payments.WebhookEvent{PaymentID: tranID, Status: "ignore"}, nil
	}
	valID := form.Get("val_id")
	if valID == "" {
		return nil, errors.New("sslcommerz: missing val_id")
	}
	q := url.Values{"val_id": {valID}, "store_id": {p.StoreID}, "store_passwd": {p.StorePasswd}, "format": {"json"}}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/validator/api/validationserverAPI.php?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.HTTP.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var v struct {
		Status   string `json:"status"`
		TranID   string `json:"tran_id"`
		Amount   string `json:"amount"`
		Currency string `json:"currency_type"`
		BankTran string `json:"bank_tran_id"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&v); err != nil {
		return nil, fmt.Errorf("sslcommerz: decode validation: %w", err)
	}
	if v.Status != "VALID" && v.Status != "VALIDATED" {
		return nil, payments.ErrBadSignature
	}
	if v.TranID != tranID {
		return nil, errors.New("sslcommerz: transaction id mismatch")
	}
	return &payments.WebhookEvent{
		PaymentID: v.TranID, ExternalID: valID, Status: "completed", Currency: v.Currency,
		AmountMinor: decimalToMinor(v.Amount), Raw: map[string]any{"bank_tran_id": v.BankTran},
	}, nil
}

func minorToDecimal(v int64) string {
	return fmt.Sprintf("%d.%02d", v/100, v%100)
}

func decimalToMinor(s string) int64 {
	whole, frac, _ := strings.Cut(strings.TrimSpace(s), ".")
	w, _ := strconv.ParseInt(whole, 10, 64)
	frac = (frac + "00")[:2]
	f, _ := strconv.ParseInt(frac, 10, 64)
	return w*100 + f
}

func nonEmpty(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
