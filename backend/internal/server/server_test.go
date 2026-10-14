package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/billing"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/config"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/token"
)

// TestEndToEnd exercises the whole stack against a real Postgres database.
// Run with TEST_DATABASE_URL=postgres://... go test ./internal/server/
func TestEndToEnd(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	resetDB(t, dsn)

	upstreamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-or-test" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/models":
			_, _ = io.WriteString(w, `{"data":[
				{"id":"openai/gpt-4o-mini","name":"OpenAI: GPT-4o mini","description":"small","created":1700000000,"context_length":128000,
				 "pricing":{"prompt":"0.00000015","completion":"0.0000006"},"top_provider":{"max_completion_tokens":16384},
				 "architecture":{"input_modalities":["text","image"]},"supported_parameters":["tools","max_tokens"]},
				{"id":"meta/llama-free:free","name":"Meta: Llama Free","created":1700000001,"context_length":8192,
				 "pricing":{"prompt":"0","completion":"0"}},
				{"id":"openrouter/auto","name":"Auto Router","pricing":{"prompt":"-1","completion":"-1"}}
			]}`)
		case "/chat/completions":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			_, _ = fmt.Fprintf(w, `{"id":"gen-1","provider":"X","model":%q,"object":"chat.completion","created":1,
				"choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],
				"usage":{"prompt_tokens":1000,"completion_tokens":1000,"total_tokens":2000}}`, body["model"])
		default:
			w.WriteHeader(404)
		}
	}))
	defer upstreamSrv.Close()

	cfg := &config.Config{
		Addr: ":0", PublicURL: "http://api.test", WebURL: "http://web.test", DatabaseURL: dsn, DBMaxConns: 8,
		InternalSecret: "internal-secret-123456", JWTSecret: "jwt-secret-1234567890", EncryptionKey: "enc-key-1234567890",
		AdminEmails: []string{"admin@example.com"}, OpenRouterKey: "sk-or-test", OpenRouterURL: upstreamSrv.URL,
		AppName: "ModelRouter", CatalogRefresh: time.Hour, ModelSyncEvery: time.Hour, FlushInterval: time.Hour,
		MaxBodyBytes: 1 << 20, UpstreamTimeout: 10 * time.Second, CORSOrigins: []string{"*"},
		Paddle: config.PaddleConfig{APIKey: "pdl", WebhookSecret: "pdl-whsec", Environment: "sandbox"},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := New(ctx, cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Pool.Close()
	srv := httptest.NewServer(app.Handler)
	defer srv.Close()
	c := &client{t: t, base: srv.URL, cfg: cfg}

	// --- Auth.js adapter creates users; admin email gets the admin role.
	var admin, user map[string]any
	c.call("POST", "/internal/auth/users", internalHdr(cfg), `{"email":"Admin@Example.com","name":"Admin"}`, 200, &admin)
	c.call("POST", "/internal/auth/users", internalHdr(cfg), `{"email":"user@example.com","name":"User"}`, 200, &user)
	if admin["role"] != "admin" || user["role"] != "user" {
		t.Fatalf("roles: %v %v", admin["role"], user["role"])
	}
	c.call("POST", "/internal/auth/users", map[string]string{"X-Internal-Secret": "wrong"}, `{"email":"x@y.z"}`, 401, nil)
	c.call("POST", "/internal/auth/accounts", internalHdr(cfg),
		fmt.Sprintf(`{"userId":%q,"provider":"github","providerAccountId":"42","type":"oauth"}`, user["id"]), 200, nil)
	var byAcct map[string]any
	c.call("GET", "/internal/auth/user-by-account?provider=github&providerAccountId=42", internalHdr(cfg), "", 200, &byAcct)
	if byAcct["id"] != user["id"] {
		t.Fatalf("user by account: %v", byAcct)
	}
	// Magic-link tokens are single use.
	c.call("POST", "/internal/auth/verification-tokens", internalHdr(cfg), `{"identifier":"user@example.com","token":"tok","expires":"2099-01-01T00:00:00Z"}`, 200, nil)
	var vt map[string]any
	c.call("POST", "/internal/auth/verification-tokens/use", internalHdr(cfg), `{"identifier":"user@example.com","token":"tok"}`, 200, &vt)
	if vt == nil {
		t.Fatal("token should be usable once")
	}
	vt = nil
	c.call("POST", "/internal/auth/verification-tokens/use", internalHdr(cfg), `{"identifier":"user@example.com","token":"tok"}`, 200, &vt)
	if vt != nil {
		t.Fatal("token reused")
	}

	adminHdr := c.session(admin["id"].(string))
	userHdr := c.session(user["id"].(string))

	// --- Non-admins cannot reach admin routes.
	c.call("GET", "/api/admin/overview", userHdr, "", 403, nil)
	c.call("GET", "/api/me", map[string]string{"Authorization": "Bearer bogus"}, "", 401, nil)

	// --- Sync upstream catalog and publish a model.
	var sync map[string]any
	c.call("POST", "/api/admin/providers/1/sync", adminHdr, "", 200, &sync)
	if sync["fetched"].(float64) != 2 { // variable-priced router skipped
		t.Fatalf("sync: %v", sync)
	}
	var pub map[string]any
	c.call("POST", "/api/admin/upstream-models/publish", adminHdr, `{"provider_id":1,"upstream_ids":["openai/gpt-4o-mini"]}`, 200, &pub)
	if pub["published"].(float64) != 1 {
		t.Fatalf("publish: %v", pub)
	}
	// Custom model: our own id and price, served by an upstream model.
	c.call("POST", "/api/admin/models", adminHdr, `{"slug":"modelrouter/swift","name":"Swift","provider_id":1,
		"upstream_id":"openai/gpt-4o-mini","pricing_mode":"manual","input_price_per_m":"1","output_price_per_m":"2",
		"request_fee":"0.00000001","max_output_tokens":512,"enabled":true,"featured":true}`, 201, nil)

	var models struct {
		Data []map[string]any `json:"data"`
	}
	c.call("GET", "/api/public/models", nil, "", 200, &models)
	if len(models.Data) != 2 || models.Data[0]["id"] != "modelrouter/swift" {
		t.Fatalf("public models: %v", models.Data)
	}
	raw := c.raw("GET", "/api/public/models", nil, "")
	if strings.Contains(raw, "upstream") || strings.Contains(raw, "provider") {
		t.Fatalf("public catalog leaks upstream details: %s", raw)
	}
	// Auto pricing = upstream * 1.10 markup: $0.15/M -> $0.165/M
	for _, m := range models.Data {
		if m["id"] == "openai/gpt-4o-mini" && m["input_price_per_m"] != "0.165" {
			t.Fatalf("markup not applied: %v", m["input_price_per_m"])
		}
	}

	// --- User creates an API key.
	var created struct {
		Key    map[string]any `json:"key"`
		Secret string         `json:"secret"`
	}
	c.call("POST", "/api/keys", userHdr, `{"name":"laptop"}`, 201, &created)
	apiKey := created.Secret
	keyID := created.Key["id"].(string)
	chat := `{"model":"modelrouter/swift","messages":[{"role":"user","content":"ping"}]}`

	// No balance yet.
	c.call("POST", "/v1/chat/completions", bearer(apiKey), chat, 402, nil)

	// --- Admin credits $5; the gateway sees it immediately (bus invalidation).
	c.call("POST", "/api/admin/users/"+user["id"].(string)+"/credit", adminHdr, `{"amount_usd":5,"note":"welcome"}`, 200, nil)
	var resp map[string]any
	c.call("POST", "/v1/chat/completions", bearer(apiKey), chat, 200, &resp)
	if resp["model"] != "modelrouter/swift" {
		t.Fatalf("response model: %v", resp["model"])
	}
	if err := app.Ledger.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	var me struct {
		User struct {
			Balance    int64 `json:"balance"`
			TotalSpent int64 `json:"total_spent"`
		} `json:"user"`
	}
	c.call("GET", "/api/me", userHdr, "", 200, &me)
	// 1000 tokens * $1/M + 1000 tokens * $2/M + 10 nano = $0.003 + 10 nano
	wantCost := int64(3_000_010)
	if me.User.Balance != 5*billing.NanoPerUSD-wantCost || me.User.TotalSpent != wantCost {
		t.Fatalf("balance %d spent %d", me.User.Balance, me.User.TotalSpent)
	}
	act := c.raw("GET", "/api/activity", userHdr, "")
	if !strings.Contains(act, `"model":"modelrouter/swift"`) || strings.Contains(act, "gpt-4o-mini") {
		t.Fatalf("activity: %s", act)
	}

	// --- Rotation invalidates the old key instantly.
	var rotated struct {
		Secret string `json:"secret"`
	}
	c.call("POST", "/api/keys/"+keyID+"/rotate", userHdr, "", 200, &rotated)
	c.call("POST", "/v1/chat/completions", bearer(apiKey), chat, 401, nil)
	c.call("POST", "/v1/chat/completions", bearer(rotated.Secret), chat, 200, nil)
	// Other users cannot touch this key.
	c.call("DELETE", "/api/keys/"+keyID, adminHdr, "", 404, nil)
	c.call("DELETE", "/api/keys/"+keyID, userHdr, "", 200, nil)
	c.call("POST", "/v1/chat/completions", bearer(rotated.Secret), chat, 401, nil)

	// --- Paddle webhook credits exactly once.
	pay, err := app.Store.CreatePayment(ctx, user["id"].(string), "paddle", "USD", 1000, 10*billing.NanoPerUSD, nil)
	if err != nil {
		t.Fatal(err)
	}
	hook := fmt.Sprintf(`{"event_type":"transaction.completed","data":{"id":"txn_1","currency_code":"USD",
		"custom_data":{"payment_id":%q},"details":{"totals":{"subtotal":"1000"}}}}`, pay.ID)
	for i := 0; i < 2; i++ {
		c.call("POST", "/webhooks/paddle", map[string]string{"Paddle-Signature": paddleSig("pdl-whsec", hook)}, hook, 200, nil)
	}
	c.call("POST", "/webhooks/paddle", map[string]string{"Paddle-Signature": paddleSig("wrong", hook)}, hook, 400, nil)
	if err := app.Ledger.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	var bal int64
	_ = app.Pool.QueryRow(ctx, `SELECT balance FROM users WHERE id=$1`, user["id"]).Scan(&bal)
	if bal != 15*billing.NanoPerUSD-2*wantCost {
		t.Fatalf("balance after top-up: %d", bal)
	}
	var methods struct {
		Methods []map[string]any `json:"methods"`
	}
	c.call("GET", "/api/billing/methods?country=BD", userHdr, "", 200, &methods)
	if len(methods.Methods) != 3 || methods.Methods[0]["currency"] != "BDT" {
		t.Fatalf("BD methods: %v", methods.Methods)
	}

	// --- Admin views.
	var overview map[string]any
	c.call("GET", "/api/admin/overview?days=7", adminHdr, "", 200, &overview)
	if overview["overview"].(map[string]any)["requests"].(float64) != 2 {
		t.Fatalf("overview: %v", overview["overview"])
	}
	c.call("PATCH", "/api/admin/users/"+admin["id"].(string), adminHdr, `{"role":"user"}`, 400, nil) // no self-demotion
	c.call("PATCH", "/api/admin/users/"+user["id"].(string), adminHdr, `{"banned":true}`, 200, nil)
	c.call("GET", "/api/me", userHdr, "", 403, nil)
	var audit struct {
		Data []map[string]any `json:"data"`
	}
	c.call("GET", "/api/admin/audit", adminHdr, "", 200, &audit)
	if len(audit.Data) < 4 {
		t.Fatalf("audit log too short: %d", len(audit.Data))
	}
	c.call("PUT", "/api/admin/settings", adminHdr, `{"default_markup_bps":2000}`, 200, nil)
	c.call("GET", "/api/public/models/openai/gpt-4o-mini", nil, "", 200, &resp)
	if resp["input_price_per_m"] != "0.18" {
		t.Fatalf("reprice after settings change: %v", resp["input_price_per_m"])
	}
	c.call("GET", "/readyz", nil, "", 200, nil)
}

func resetDB(t *testing.T, dsn string) {
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
}

type client struct {
	t    *testing.T
	base string
	cfg  *config.Config
}

func (c *client) raw(method, path string, hdr map[string]string, body string) string {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.base+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func (c *client) call(method, path string, hdr map[string]string, body string, want int, out any) {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.base+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		c.t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, want, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			c.t.Fatalf("%s %s: decode: %v: %s", method, path, err, b)
		}
	}
}

func (c *client) session(userID string) map[string]string {
	tok, _ := token.Sign([]byte(c.cfg.JWTSecret), token.Claims{Subject: userID, ExpiresAt: time.Now().Add(time.Minute).Unix(), Audience: token.Audience})
	return map[string]string{"Authorization": "Bearer " + tok}
}

func internalHdr(cfg *config.Config) map[string]string {
	return map[string]string{"X-Internal-Secret": cfg.InternalSecret}
}

func bearer(k string) map[string]string { return map[string]string{"Authorization": "Bearer " + k} }

func paddleSig(secret, body string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(ts + ":" + body))
	return "ts=" + ts + ";h1=" + hex.EncodeToString(m.Sum(nil))
}
