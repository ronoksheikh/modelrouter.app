package token

import (
	"strings"
	"testing"
	"time"
)

func TestSignVerify(t *testing.T) {
	secret := []byte("0123456789abcdef0123")
	now := time.Now()
	tok, err := Sign(secret, Claims{Subject: "u1", ExpiresAt: now.Add(time.Minute).Unix(), Audience: Audience})
	if err != nil {
		t.Fatal(err)
	}
	c, err := Verify(secret, tok, now)
	if err != nil || c.Subject != "u1" {
		t.Fatalf("verify: %v %+v", err, c)
	}
	if _, err := Verify([]byte("wrong-secret-wrong"), tok, now); err != ErrSignature {
		t.Fatalf("wrong secret: %v", err)
	}
	if _, err := Verify(secret, tok, now.Add(5*time.Minute)); err != ErrExpired {
		t.Fatalf("expired: %v", err)
	}
	parts := strings.Split(tok, ".")
	if _, err := Verify(secret, parts[0]+"."+parts[1]+"x."+parts[2], now); err == nil {
		t.Fatal("tampered payload accepted")
	}
	// alg=none must be rejected.
	none := b64.EncodeToString([]byte(`{"alg":"none"}`)) + "." + parts[1] + "."
	if _, err := Verify(secret, none, now); err == nil {
		t.Fatal("alg none accepted")
	}
}
