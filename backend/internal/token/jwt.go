// Package token verifies the short-lived HS256 tokens the web server mints when
// it calls the backend on behalf of a signed-in user.
package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrMalformed = errors.New("malformed token")
	ErrSignature = errors.New("invalid token signature")
	ErrExpired   = errors.New("token expired")
)

type Claims struct {
	Subject   string `json:"sub"`
	Email     string `json:"email,omitempty"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Audience  string `json:"aud,omitempty"`
}

const Audience = "modelrouter-api"

var b64 = base64.RawURLEncoding

func Sign(secret []byte, c Claims) (string, error) {
	header := b64.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	unsigned := header + "." + b64.EncodeToString(payload)
	return unsigned + "." + b64.EncodeToString(mac(secret, unsigned)), nil
}

func Verify(secret []byte, tok string, now time.Time) (*Claims, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return nil, ErrMalformed
	}
	hb, err := b64.DecodeString(parts[0])
	if err != nil {
		return nil, ErrMalformed
	}
	var h struct {
		Alg string `json:"alg"`
	}
	if json.Unmarshal(hb, &h) != nil || h.Alg != "HS256" {
		return nil, ErrMalformed
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil {
		return nil, ErrMalformed
	}
	if !hmac.Equal(sig, mac(secret, parts[0]+"."+parts[1])) {
		return nil, ErrSignature
	}
	pb, err := b64.DecodeString(parts[1])
	if err != nil {
		return nil, ErrMalformed
	}
	var c Claims
	if err := json.Unmarshal(pb, &c); err != nil {
		return nil, ErrMalformed
	}
	// Allow small clock skew between web and API servers.
	if c.ExpiresAt == 0 || now.Unix() > c.ExpiresAt+30 {
		return nil, ErrExpired
	}
	if c.Audience != "" && c.Audience != Audience {
		return nil, ErrMalformed
	}
	if c.Subject == "" {
		return nil, ErrMalformed
	}
	return &c, nil
}

func mac(secret []byte, s string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(s))
	return m.Sum(nil)
}
