// Package session authenticates dashboard/admin requests made by the web
// server on behalf of a signed-in user, and server-to-server calls.
package session

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/cache"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/httpx"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/token"
)

type Users interface {
	GetUser(ctx context.Context, id string) (*store.User, error)
}

type Auth struct {
	JWTSecret      []byte
	InternalSecret string
	users          Users
	cache          *cache.TTL[*store.User]
}

func New(jwtSecret, internalSecret string, users Users) *Auth {
	return &Auth{JWTSecret: []byte(jwtSecret), InternalSecret: internalSecret, users: users, cache: cache.NewTTL[*store.User](50_000)}
}

type UserHandler func(w http.ResponseWriter, r *http.Request, u *store.User)

func (a *Auth) resolve(r *http.Request) (*store.User, int, string) {
	h := r.Header.Get("Authorization")
	if len(h) < 8 || !strings.EqualFold(h[:7], "bearer ") {
		return nil, 401, "missing session token"
	}
	claims, err := token.Verify(a.JWTSecret, strings.TrimSpace(h[7:]), time.Now())
	if err != nil {
		return nil, 401, "invalid session token"
	}
	if u, ok := a.cache.Get(claims.Subject); ok {
		return u, 0, ""
	}
	u, err := a.users.GetUser(r.Context(), claims.Subject)
	if errors.Is(err, store.ErrNotFound) {
		return nil, 401, "user not found"
	}
	if err != nil {
		return nil, 503, "temporarily unavailable"
	}
	a.cache.Set(u.ID, u, 15*time.Second)
	return u, 0, ""
}

// User requires a valid user session.
func (a *Auth) User(next UserHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, status, msg := a.resolve(r)
		if u == nil {
			httpx.Error(w, status, msg)
			return
		}
		if u.Banned {
			httpx.Error(w, 403, "account suspended")
			return
		}
		next(w, r, u)
	}
}

// Admin requires a session whose user currently has the admin role (read from
// the database, never from the token).
func (a *Auth) Admin(next UserHandler) http.HandlerFunc {
	return a.User(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		if u.Role != "admin" {
			httpx.Error(w, 403, "admin access required")
			return
		}
		next(w, r, u)
	})
}

// Internal requires the shared server-to-server secret.
func (a *Auth) Internal(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Internal-Secret")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(a.InternalSecret)) != 1 {
			httpx.Error(w, 401, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (a *Auth) Invalidate(userID string) { a.cache.Delete(userID) }
