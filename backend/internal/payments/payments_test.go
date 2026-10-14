package payments

import (
	"context"
	"net/http"
	"testing"
)

type stub struct{ m Method }

func (s stub) ID() string     { return s.m.ID }
func (s stub) Method() Method { return s.m }
func (s stub) CreateCheckout(context.Context, CheckoutRequest) (*Checkout, error) {
	return &Checkout{}, nil
}
func (s stub) ParseWebhook(context.Context, *http.Request, []byte) (*WebhookEvent, error) {
	return nil, nil
}

func TestMethodsForRegion(t *testing.T) {
	r := NewRegistry()
	r.Register(stub{Method{ID: "paddle", Regions: []string{RegionGlobal}}})
	r.Register(stub{Method{ID: "bdpay", Regions: []string{RegionBD}}})
	r.AddUpcoming(Method{ID: "nagad", Regions: []string{RegionBD}})

	bd := r.MethodsFor("bd")
	if len(bd) != 3 || bd[0].ID != "bdpay" || bd[1].ID != "nagad" || bd[2].ID != "paddle" {
		t.Fatalf("BD users should see local methods first: %+v", bd)
	}
	if bd[1].Available {
		t.Fatal("upcoming methods must not be available")
	}
	us := r.MethodsFor("US")
	if len(us) != 1 || us[0].ID != "paddle" {
		t.Fatalf("international users: %+v", us)
	}
	if r.AllowedFor("bdpay", "US") || !r.AllowedFor("bdpay", "BD") || !r.AllowedFor("paddle", "BD") || r.AllowedFor("nagad", "BD") {
		t.Fatal("AllowedFor wrong")
	}
}
