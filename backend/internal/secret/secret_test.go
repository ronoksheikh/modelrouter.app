package secret

import (
	"strings"
	"testing"
)

func TestBox(t *testing.T) {
	b, _ := NewBox("encryption-key-123")
	enc, err := b.Encrypt("sk-or-v1-secret")
	if err != nil || !strings.HasPrefix(enc, "v1:") || strings.Contains(enc, "secret") {
		t.Fatalf("encrypt: %q %v", enc, err)
	}
	dec, err := b.Decrypt(enc)
	if err != nil || dec != "sk-or-v1-secret" {
		t.Fatalf("decrypt: %q %v", dec, err)
	}
	other, _ := NewBox("another-key-456789")
	if _, err := other.Decrypt(enc); err == nil {
		t.Fatal("decrypted with wrong key")
	}
	if e, _ := b.Encrypt(""); e != "" {
		t.Fatal("empty should stay empty")
	}
}

func TestAPIKey(t *testing.T) {
	k1, _ := NewAPIKey()
	k2, _ := NewAPIKey()
	if k1 == k2 || !strings.HasPrefix(k1, KeyPrefix) || len(k1) != len(KeyPrefix)+48 {
		t.Fatalf("bad keys %q %q", k1, k2)
	}
	if HashKey(k1) == HashKey(k2) || len(HashKey(k1)) != 64 {
		t.Fatal("hash")
	}
	if p := DisplayPrefix(k1); len(p) != len(KeyPrefix)+8 || !strings.HasPrefix(k1, p) {
		t.Fatalf("prefix %q", p)
	}
}
