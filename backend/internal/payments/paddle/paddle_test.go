package paddle

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/payments"
)

func sign(secret string, ts int64, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(strconv.FormatInt(ts, 10) + ":" + body))
	return "ts=" + strconv.FormatInt(ts, 10) + ";h1=" + hex.EncodeToString(m.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	body := []byte(`{"a":1}`)
	good := sign("whsec", now.Unix(), string(body))
	if err := VerifySignature("whsec", good, body, now, 5*time.Minute); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := VerifySignature("other", good, body, now, 5*time.Minute); err == nil {
		t.Fatal("wrong secret accepted")
	}
	if err := VerifySignature("whsec", good, []byte(`{"a":2}`), now, 5*time.Minute); err == nil {
		t.Fatal("tampered body accepted")
	}
	if err := VerifySignature("whsec", good, body, now.Add(10*time.Minute), 5*time.Minute); err == nil {
		t.Fatal("replayed (old) webhook accepted")
	}
	if err := VerifySignature("whsec", "garbage", body, now, 5*time.Minute); err == nil {
		t.Fatal("garbage header accepted")
	}
}

func TestParseWebhook(t *testing.T) {
	p := New("key", "whsec", "sandbox", "")
	now := time.Now()
	p.Now = func() time.Time { return now }
	body := `{"event_type":"transaction.completed","data":{"id":"txn_1","status":"completed","currency_code":"USD",
		"custom_data":{"payment_id":"pay-123"},"details":{"totals":{"subtotal":"1000","grand_total":"1150"}}}}`
	req := httptest.NewRequest("POST", "/webhooks/paddle", strings.NewReader(body))
	req.Header.Set("Paddle-Signature", sign("whsec", now.Unix(), body))
	ev, err := p.ParseWebhook(context.Background(), req, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Status != "completed" || ev.PaymentID != "pay-123" || ev.ExternalID != "txn_1" || ev.AmountMinor != 1000 || ev.Currency != "USD" {
		t.Fatalf("event: %+v", ev)
	}
	req.Header.Set("Paddle-Signature", sign("nope", now.Unix(), body))
	if _, err := p.ParseWebhook(context.Background(), req, []byte(body)); err == nil {
		t.Fatal("bad signature accepted")
	}
}

func TestCreateCheckout(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transactions" || r.Header.Get("Authorization") != "Bearer key" {
			w.WriteHeader(401)
			return
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = io.WriteString(w, `{"data":{"id":"txn_9","checkout":{"url":"https://pay.example/?_ptxn=txn_9"}}}`)
	}))
	defer srv.Close()
	p := New("key", "whsec", "sandbox", "").WithBaseURL(srv.URL)
	co, err := p.CreateCheckout(context.Background(), payments.CheckoutRequest{
		PaymentID: "pay-1", UserID: "u1", Currency: "USD", AmountMinor: 2500, SuccessURL: "https://modelrouter.app/dashboard/billing",
	})
	if err != nil {
		t.Fatal(err)
	}
	if co.ExternalID != "txn_9" || co.RedirectURL == "" || co.ClientData["transaction_id"] != "txn_9" {
		t.Fatalf("checkout: %+v", co)
	}
	items := got["items"].([]any)
	price := items[0].(map[string]any)["price"].(map[string]any)
	if price["unit_price"].(map[string]any)["amount"] != "2500" {
		t.Fatalf("amount not sent: %v", price)
	}
	if got["custom_data"].(map[string]any)["payment_id"] != "pay-1" {
		t.Fatalf("custom data: %v", got["custom_data"])
	}
}
