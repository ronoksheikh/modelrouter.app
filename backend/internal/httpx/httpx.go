// Package httpx contains HTTP helpers and middleware shared by all handlers.
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

type ctxKey int

const requestIDKey ctxKey = 1

func NewID(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}

func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes a JSON error in the shape used by the dashboard/admin APIs.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]any{"error": map[string]any{"message": msg, "code": status}})
}

// OpenAIError writes an error shaped like the OpenAI API.
func OpenAIError(w http.ResponseWriter, status int, typ, msg string) {
	JSON(w, status, map[string]any{"error": map[string]any{"message": msg, "type": typ, "code": status}})
}

// AnthropicError writes an error shaped like the Anthropic API.
func AnthropicError(w http.ResponseWriter, status int, typ, msg string) {
	JSON(w, status, map[string]any{"type": "error", "error": map[string]any{"type": typ, "message": msg}})
}

// AnthropicErrorType maps an HTTP status to Anthropic's error type names.
func AnthropicErrorType(status int) string {
	switch status {
	case 400:
		return "invalid_request_error"
	case 401:
		return "authentication_error"
	case 402:
		return "billing_error"
	case 403:
		return "permission_error"
	case 404:
		return "not_found_error"
	case 413:
		return "request_too_large"
	case 429:
		return "rate_limit_error"
	case 503, 529:
		return "overloaded_error"
	default:
		if status >= 500 {
			return "api_error"
		}
		return "invalid_request_error"
	}
}

// OpenAIErrorType maps an HTTP status to OpenAI's error type names.
func OpenAIErrorType(status int) string {
	switch status {
	case 401:
		return "authentication_error"
	case 402:
		return "insufficient_quota"
	case 403:
		return "permission_error"
	case 404:
		return "not_found_error"
	case 429:
		return "rate_limit_exceeded"
	default:
		if status >= 500 {
			return "server_error"
		}
		return "invalid_request_error"
	}
}

var ErrBodyTooLarge = errors.New("request body too large")

// DecodeJSON decodes a JSON body with a size limit.
func DecodeJSON(r *http.Request, limit int64, v any) error {
	body, err := ReadBody(r, limit)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func ReadBody(r *http.Request, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, ErrBodyTooLarge
	}
	return body, nil
}

// statusWriter records the status code while preserving Flusher support.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Middleware chain: recover → request id → access log.
func Wrap(h http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 64 {
			id = NewID("req_")
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		sw := &statusWriter{ResponseWriter: w}
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				log.Error("panic", "err", rec, "path", r.URL.Path, "stack", string(debug.Stack()))
				if sw.status == 0 {
					Error(sw, http.StatusInternalServerError, "internal server error")
				}
			}
			lvl := slog.LevelDebug
			if sw.status >= 500 {
				lvl = slog.LevelWarn
			}
			log.Log(ctx, lvl, "http", "method", r.Method, "path", r.URL.Path, "status", sw.status,
				"dur_ms", time.Since(start).Milliseconds(), "req_id", id)
		}()
		h.ServeHTTP(sw, r.WithContext(ctx))
	})
}

// CORS allows browser clients to call the public API directly.
func CORS(h http.Handler, origins []string) http.Handler {
	allowAll := len(origins) == 0 || (len(origins) == 1 && origins[0] == "*")
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (allowAll || allowed[strings.ToLower(origin)]) {
			hdr := w.Header()
			if allowAll {
				hdr.Set("Access-Control-Allow-Origin", "*")
			} else {
				hdr.Set("Access-Control-Allow-Origin", origin)
				hdr.Add("Vary", "Origin")
			}
			hdr.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Api-Key, Anthropic-Version, Anthropic-Beta, X-Title, HTTP-Referer, X-Stainless-Lang, X-Stainless-Package-Version, X-Stainless-OS, X-Stainless-Arch, X-Stainless-Runtime, X-Stainless-Runtime-Version, X-Stainless-Retry-Count, X-Stainless-Timeout, Anthropic-Dangerous-Direct-Browser-Access")
			hdr.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			hdr.Set("Access-Control-Max-Age", "86400")
			hdr.Set("Access-Control-Expose-Headers", "X-Request-Id")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// ClientIP returns the best-effort client IP (trusting the first X-Forwarded-For hop).
func ClientIP(r *http.Request) string {
	if v := r.Header.Get("CF-Connecting-IP"); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i >= 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host
}

func QueryInt(r *http.Request, key string, def, min, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
