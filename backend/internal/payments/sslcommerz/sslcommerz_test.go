package sslcommerz

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/payments"
)

func TestFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gwprocess/v4/api.php":
			_ = r.ParseForm()
			if r.Form.Get("total_amount") != "1220.00" || r.Form.Get("currency") != "BDT" || r.Form.Get("tran_id") != "pay-1" {
				_, _ = io.WriteString(w, `{"status":"FAILED","failedreason":"bad form"}`)
				return
			}
			_, _ = io.WriteString(w, `{"status":"SUCCESS","sessionkey":"S1","GatewayPageURL":"https://gw/S1"}`)
		case "/validator/api/validationserverAPI.php":
			if r.URL.Query().Get("val_id") == "good" {
				_, _ = io.WriteString(w, `{"status":"VALID","tran_id":"pay-1","amount":"1220.00","currency_type":"BDT"}`)
				return
			}
			_, _ = io.WriteString(w, `{"status":"INVALID_TRANSACTION"}`)
		}
	}))
	defer srv.Close()
	p := New("store", "pass", true).WithBaseURL(srv.URL)

	co, err := p.CreateCheckout(context.Background(), payments.CheckoutRequest{PaymentID: "pay-1", Currency: "BDT", AmountMinor: 122000, Email: "a@b.c"})
	if err != nil || co.RedirectURL != "https://gw/S1" {
		t.Fatalf("checkout: %+v %v", co, err)
	}

	ipn := url.Values{"status": {"VALID"}, "tran_id": {"pay-1"}, "val_id": {"good"}}.Encode()
	ev, err := p.ParseWebhook(context.Background(), httptest.NewRequest("POST", "/", strings.NewReader(ipn)), []byte(ipn))
	if err != nil || ev.Status != "completed" || ev.AmountMinor != 122000 || ev.PaymentID != "pay-1" {
		t.Fatalf("ipn: %+v %v", ev, err)
	}

	// A forged IPN is rejected by server-side validation.
	forged := url.Values{"status": {"VALID"}, "tran_id": {"pay-1"}, "val_id": {"forged"}}.Encode()
	if _, err := p.ParseWebhook(context.Background(), httptest.NewRequest("POST", "/", strings.NewReader(forged)), []byte(forged)); err == nil {
		t.Fatal("forged IPN accepted")
	}
}

func TestDecimal(t *testing.T) {
	if decimalToMinor("12.5") != 1250 || decimalToMinor("7") != 700 || minorToDecimal(1205) != "12.05" {
		t.Fatal("decimal conversion")
	}
}
