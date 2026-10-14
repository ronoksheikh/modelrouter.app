// Package secret encrypts small secrets (provider API keys) at rest with AES-GCM
// and generates/hashes user API keys.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
)

type Box struct{ aead cipher.AEAD }

func NewBox(key string) (*Box, error) {
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

func (b *Box) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return "v1:" + base64.StdEncoding.EncodeToString(out), nil
}

func (b *Box) Decrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	if len(enc) < 3 || enc[:3] != "v1:" {
		return "", errors.New("unknown secret format")
	}
	raw, err := base64.StdEncoding.DecodeString(enc[3:])
	if err != nil {
		return "", err
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	plain, err := b.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// KeyPrefix is prepended to every user API key.
const KeyPrefix = "mr-sk-"

// NewAPIKey returns a new random API key with ~285 bits of entropy.
func NewAPIKey() (string, error) {
	s, err := RandomString(48)
	if err != nil {
		return "", err
	}
	return KeyPrefix + s, nil
}

// RandomString returns n random base62 characters.
func RandomString(n int) (string, error) {
	buf := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))
	for i := range buf {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		buf[i] = alphabet[v.Int64()]
	}
	return string(buf), nil
}

// HashKey returns the hex SHA-256 of an API key. Keys are high-entropy random
// strings, so a fast hash is appropriate (no need for a password KDF).
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// DisplayPrefix is the part of the key we keep for display, e.g. "mr-sk-a1b2c3d4".
func DisplayPrefix(key string) string {
	if len(key) <= len(KeyPrefix)+8 {
		return key
	}
	return key[:len(KeyPrefix)+8]
}
