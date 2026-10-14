package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/billing"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/catalog"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/keyauth"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/ratelimit"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/upstream"
)

const testKey = "mr-sk-testtesttesttesttesttesttest"

type fakeKeys struct{ auth *store.KeyAuth }

func (f *fakeKeys) Lookup(_ context.Context, raw string) (*store.KeyAuth, error) {
	if raw != testKey {
		return nil, keyauth.ErrInvalidKey
	}
	return f.auth, nil
}

type fakeLedger struct {
	mu      sync.Mutex
	balance int64
	recs    []store.UsageRecord
}

func (f *fakeLedger) Available(context.Context, string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.balance, nil
}
func (f *fakeLedger) PendingKeySpend(string, string) int64 { return 0 }
func (f *fakeLedger) Record(r store.UsageRecord) {
	f.mu.Lock()
	f.recs = append(f.recs, r)
	f.balance -= r.Cost
	f.mu.Unlock()
}
func (f *fakeLedger) last(t *testing.T) store.UsageRecord {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.recs) == 0 {
		t.Fatal("no usage recorded")
	}
	return f.recs[len(f.recs)-1]
}

// upstreamRecorder captures what the gateway sent upstream.
type upstreamRecorder struct {
	mu     sync.Mutex
	paths  []string
	bodies []map[string]any
	calls  atomic.Int32
}

func (u *upstreamRecorder) capture(r *http.Request) map[string]any {
	b, _ := io.ReadAll(r.Body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	u.mu.Lock()
	u.paths = append(u.paths, r.URL.Path)
	u.bodies = append(u.bodies, m)
	u.mu.Unlock()
	u.calls.Add(1)
	return m
}

func (u *upstreamRecorder) lastBody() map[string]any {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.bodies[len(u.bodies)-1]
}

// fakeOpenRouter mimics an OpenAI-compatible upstream that leaks its identity.
func fakeOpenRouter(rec *upstreamRecorder, failFirst int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := rec.capture(r)
		if rec.calls.Load() <= failFirst {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":{"message":"OpenRouter: provider overloaded"}}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer upstream-secret" {
			w.WriteHeader(401)
			return
		}
		if body["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			fl := w.(http.Flusher)
			for _, c := range []string{
				": OPENROUTER PROCESSING",
				`data: {"id":"gen-123","provider":"OpenAI","model":"openai/gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"}}]}`,
				`data: {"id":"gen-123","provider":"OpenAI","model":"openai/gpt-4o","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop","native_finish_reason":"stop"}]}`,
				`data: {"id":"gen-123","provider":"OpenAI","model":"openai/gpt-4o","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cost":0.0004}}`,
				`data: [DONE]`,
			} {
				_, _ = io.WriteString(w, c+"\n\n")
				fl.Flush()
			}
			return
		}
		_, _ = io.WriteString(w, `{"id":"gen-123","provider":"OpenAI","model":"openai/gpt-4o","object":"chat.completion","created":1,
			"choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop","native_finish_reason":"stop"}],
			"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cost":0.0004,"is_byok":false}}`)
	}))
}

// fakeAnthropic mimics Anthropic's native Messages API.
func fakeAnthropic(rec *upstreamRecorder) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := rec.capture(r)
		if r.Header.Get("x-api-key") != "anth-secret" || r.Header.Get("anthropic-version") == "" {
			w.WriteHeader(401)
			return
		}
		if body["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			for _, e := range []string{
				"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_up\",\"model\":\"claude-upstream\",\"usage\":{\"input_tokens\":40,\"output_tokens\":1}}}",
				"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}",
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi there\"}}",
				"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}",
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":20}}",
				"event: message_stop\ndata: {\"type\":\"message_stop\"}",
			} {
				_, _ = io.WriteString(w, e+"\n\n")
			}
			return
		}
		_, _ = io.WriteString(w, `{"id":"msg_up","type":"message","role":"assistant","model":"claude-upstream",
			"content":[{"type":"text","text":"Hi there"}],"stop_reason":"end_turn","usage":{"input_tokens":40,"output_tokens":20}}`)
	}))
}

type env struct {
	h       *Handler
	ledger  *fakeLedger
	keys    *fakeKeys
	mux     *http.ServeMux
	orRec   *upstreamRecorder
	anRec   *upstreamRecorder
	cleanup func()
}

func newEnv(t *testing.T, failFirst int32, settings func(*store.Settings)) *env {
	t.Helper()
	orRec, anRec := &upstreamRecorder{}, &upstreamRecorder{}
	or := fakeOpenRouter(orRec, failFirst)
	an := fakeAnthropic(anRec)
	st := store.DefaultSettings()
	st.DefaultRequestFee = 10
	if settings != nil {
		settings(&st)
	}
	providers := []*upstream.Provider{
		{ID: 1, Name: "or", Kind: upstream.KindOpenAI, BaseURL: or.URL, APIKey: "upstream-secret", IsOpenRouter: true},
		{ID: 2, Name: "anth", Kind: upstream.KindAnthropic, BaseURL: an.URL, APIKey: "anth-secret"},
	}
	models := []store.Model{
		{Slug: "modelrouter/fast", Name: "Fast", ProviderID: 1, UpstreamID: "openai/gpt-4o", Enabled: true,
			InputPrice: 2 * billing.NanoPerUSD, OutputPrice: 8 * billing.NanoPerUSD, MaxOutputTokens: 1000,
			UpstreamInputPrice: billing.NanoPerUSD, UpstreamOutputPrice: 4 * billing.NanoPerUSD},
		{Slug: "modelrouter/free", Name: "Free", ProviderID: 1, UpstreamID: "meta/llama:free", Enabled: true},
		{Slug: "claude-native", Name: "Claude", ProviderID: 2, UpstreamID: "claude-upstream", Enabled: true,
			InputPrice: 3 * billing.NanoPerUSD, OutputPrice: 15 * billing.NanoPerUSD},
		{Slug: "fallback-model", Name: "FB", ProviderID: 1, UpstreamID: "a/primary", FallbackUpstream: []string{"b/secondary"}, Enabled: true,
			InputPrice: 1, OutputPrice: 1},
		{Slug: "disabled", Name: "Off", ProviderID: 1, UpstreamID: "x", Enabled: false},
	}
	led := &fakeLedger{balance: 10 * billing.NanoPerUSD}
	keys := &fakeKeys{auth: &store.KeyAuth{KeyID: "key-1", UserID: "user-1"}}
	h := &Handler{
		Keys: keys, Catalog: catalog.NewStatic(catalog.Build(st, providers, models)), Ledger: led,
		Upstream: upstream.NewClient(10*time.Second, "ModelRouter", "https://modelrouter.app"),
		Limiter:  ratelimit.New(), Log: slog.New(slog.NewTextHandler(io.Discard, nil)), MaxBody: 1 << 20,
		DefaultAnthropicMaxTokens: 4096,
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return &env{h: h, ledger: led, keys: keys, mux: mux, orRec: orRec, anRec: anRec, cleanup: func() { or.Close(); an.Close() }}
}

func (e *env) do(t *testing.T, method, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testKey)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	e.mux.ServeHTTP(rr, req)
	return rr
}

func TestOpenAIPassthroughHidesUpstream(t *testing.T) {
	e := newEnv(t, 0, nil)
	defer e.cleanup()
	rr := e.do(t, "POST", "/v1/chat/completions",
		`{"model":"modelrouter/fast","messages":[{"role":"user","content":"hi"}],"max_tokens":99999,"provider":{"order":["x"]},"models":["y"]}`, nil)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body)
	}
	out := rr.Body.String()
	for _, leak := range []string{"openai/gpt-4o", "OpenAI", "gen-123", "cost", "native_finish_reason", "is_byok"} {
		if strings.Contains(out, leak) {
			t.Errorf("response leaks %q: %s", leak, out)
		}
	}
	if !strings.Contains(out, `"model":"modelrouter/fast"`) || !strings.Contains(out, `"prompt_tokens":100`) {
		t.Fatalf("unexpected body: %s", out)
	}
	sent := e.orRec.lastBody()
	if sent["model"] != "openai/gpt-4o" {
		t.Fatalf("upstream model = %v", sent["model"])
	}
	if sent["max_tokens"] != float64(1000) {
		t.Fatalf("token cap not applied: %v", sent["max_tokens"])
	}
	if _, ok := sent["provider"]; ok {
		t.Fatal("routing fields must be stripped")
	}
	if _, ok := sent["models"]; ok {
		t.Fatal("routing fields must be stripped")
	}
	rec := e.ledger.last(t)
	// 100 in * $2/M + 50 out * $8/M + 10 nano fee = 200_000 + 400_000 + 10
	if rec.Cost != 600_010 || rec.InputTokens != 100 || rec.OutputTokens != 50 || rec.Status != 200 {
		t.Fatalf("usage record wrong: %+v", rec)
	}
	if rec.UpstreamCost != 400_000 { // reported cost $0.0004
		t.Fatalf("upstream cost = %d", rec.UpstreamCost)
	}
}

func TestOpenAIStreamPassthrough(t *testing.T) {
	e := newEnv(t, 0, nil)
	defer e.cleanup()
	rr := e.do(t, "POST", "/v1/chat/completions",
		`{"model":"modelrouter/fast","stream":true,"messages":[{"role":"user","content":"hi"}]}`, nil)
	if rr.Code != 200 || rr.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d %s", rr.Code, rr.Header().Get("Content-Type"))
	}
	out := rr.Body.String()
	if strings.Contains(out, "OPENROUTER") || strings.Contains(out, "openai/gpt-4o") || strings.Contains(out, `"provider"`) {
		t.Fatalf("stream leaks upstream: %s", out)
	}
	// Client did not ask for usage, so the usage-only chunk is dropped...
	if strings.Contains(out, `"usage"`) {
		t.Fatalf("unrequested usage chunk forwarded: %s", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "data: [DONE]") {
		t.Fatalf("missing DONE: %s", out)
	}
	// ...but still billed.
	if rec := e.ledger.last(t); rec.InputTokens != 100 || rec.OutputTokens != 50 || !rec.Stream {
		t.Fatalf("usage %+v", rec)
	}
	if so, _ := e.orRec.lastBody()["stream_options"].(map[string]any); so["include_usage"] != true {
		t.Fatal("gateway must request usage from upstream")
	}

	rr = e.do(t, "POST", "/v1/chat/completions",
		`{"model":"modelrouter/fast","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`, nil)
	if !strings.Contains(rr.Body.String(), `"prompt_tokens":100`) || strings.Contains(rr.Body.String(), `"cost"`) {
		t.Fatalf("requested usage missing or cost leaked: %s", rr.Body)
	}
}

func TestAnthropicEndpointOverOpenAIUpstream(t *testing.T) {
	e := newEnv(t, 0, nil)
	defer e.cleanup()
	hdr := map[string]string{"Authorization": "", "X-Api-Key": testKey, "Anthropic-Version": "2023-06-01"}
	rr := e.do(t, "POST", "/v1/messages",
		`{"model":"modelrouter/fast","max_tokens":50,"system":"be brief","messages":[{"role":"user","content":"hi"}]}`, hdr)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body)
	}
	var resp struct {
		Type    string `json:"type"`
		Model   string `json:"model"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Type != "message" || resp.Model != "modelrouter/fast" || resp.Content[0].Text != "Hello" || resp.StopReason != "end_turn" || resp.Usage.InputTokens != 100 {
		t.Fatalf("bad translation: %s", rr.Body)
	}
	sent := e.orRec.lastBody()
	msgs := sent["messages"].([]any)
	if sent["model"] != "openai/gpt-4o" || len(msgs) != 2 || msgs[0].(map[string]any)["role"] != "system" {
		t.Fatalf("bad upstream request: %v", sent)
	}

	// Streaming.
	rr = e.do(t, "POST", "/v1/messages",
		`{"model":"modelrouter/fast","max_tokens":50,"stream":true,"messages":[{"role":"user","content":"hi"}]}`, hdr)
	out := rr.Body.String()
	for _, want := range []string{"event: message_start", `"text":"Hel"`, "event: message_stop", `"output_tokens":50`} {
		if !strings.Contains(out, want) {
			t.Fatalf("anthropic stream missing %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[DONE]") || strings.Contains(out, "OPENROUTER") {
		t.Fatalf("stream leaks: %s", out)
	}
}

func TestOpenAIEndpointOverAnthropicUpstream(t *testing.T) {
	e := newEnv(t, 0, nil)
	defer e.cleanup()
	rr := e.do(t, "POST", "/v1/chat/completions",
		`{"model":"claude-native","messages":[{"role":"system","content":"s"},{"role":"user","content":"hi"}]}`, nil)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), `"content":"Hi there"`) || !strings.Contains(rr.Body.String(), `"model":"claude-native"`) {
		t.Fatalf("bad: %s", rr.Body)
	}
	sent := e.anRec.lastBody()
	if sent["model"] != "claude-upstream" || sent["max_tokens"] != float64(4096) || sent["system"] != "s" {
		t.Fatalf("bad upstream body: %v", sent)
	}
	if e.anRec.paths[len(e.anRec.paths)-1] != "/messages" {
		t.Fatalf("wrong path %v", e.anRec.paths)
	}
	rec := e.ledger.last(t)
	if rec.InputTokens != 40 || rec.OutputTokens != 20 {
		t.Fatalf("usage %+v", rec)
	}

	rr = e.do(t, "POST", "/v1/chat/completions",
		`{"model":"claude-native","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`, nil)
	out := rr.Body.String()
	for _, want := range []string{`"content":"Hi there"`, `"finish_reason":"stop"`, `"completion_tokens":20`, "data: [DONE]"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestAnthropicNativePassthroughStream(t *testing.T) {
	e := newEnv(t, 0, nil)
	defer e.cleanup()
	rr := e.do(t, "POST", "/v1/messages",
		`{"model":"claude-native","max_tokens":10,"stream":true,"messages":[{"role":"user","content":"hi"}]}`, nil)
	out := rr.Body.String()
	if strings.Contains(out, "claude-upstream") || !strings.Contains(out, `"model":"claude-native"`) {
		t.Fatalf("model not rewritten: %s", out)
	}
	rec := e.ledger.last(t)
	if rec.InputTokens != 40 || rec.OutputTokens != 20 {
		t.Fatalf("usage %+v", rec)
	}
}

func TestAdmissionErrors(t *testing.T) {
	e := newEnv(t, 0, nil)
	defer e.cleanup()
	body := `{"model":"modelrouter/fast","messages":[{"role":"user","content":"hi"}]}`

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	rr := httptest.NewRecorder()
	e.mux.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("no key: %d", rr.Code)
	}
	rr = e.do(t, "POST", "/v1/chat/completions", body, map[string]string{"Authorization": "Bearer mr-sk-wrongwrongwrongwrongwrong"})
	if rr.Code != 401 {
		t.Fatalf("bad key: %d", rr.Code)
	}
	rr = e.do(t, "POST", "/v1/chat/completions", `{"model":"nope","messages":[]}`, nil)
	if rr.Code != 404 {
		t.Fatalf("unknown model: %d", rr.Code)
	}
	rr = e.do(t, "POST", "/v1/chat/completions", `{"model":"disabled","messages":[]}`, nil)
	if rr.Code != 404 {
		t.Fatalf("disabled model: %d", rr.Code)
	}
	rr = e.do(t, "POST", "/v1/messages", `{"model":"modelrouter/fast","messages":[{"role":"user","content":"x"}]}`, nil)
	if rr.Code != 400 || !strings.Contains(rr.Body.String(), `"type":"error"`) {
		t.Fatalf("anthropic missing max_tokens: %d %s", rr.Code, rr.Body)
	}

	e.ledger.balance = 0
	rr = e.do(t, "POST", "/v1/chat/completions", body, nil)
	if rr.Code != 402 {
		t.Fatalf("no balance: %d", rr.Code)
	}
	e.ledger.balance = billing.NanoPerUSD
	e.keys.auth.Disabled = true
	if rr = e.do(t, "POST", "/v1/chat/completions", body, nil); rr.Code != 401 {
		t.Fatalf("disabled key: %d", rr.Code)
	}
	e.keys.auth.Disabled = false
	e.keys.auth.UserBanned = true
	if rr = e.do(t, "POST", "/v1/chat/completions", body, nil); rr.Code != 403 {
		t.Fatalf("banned: %d", rr.Code)
	}
	e.keys.auth.UserBanned = false
	limit := int64(5)
	e.keys.auth.SpendLimit = &limit
	e.keys.auth.KeySpent = 5
	if rr = e.do(t, "POST", "/v1/chat/completions", body, nil); rr.Code != 402 {
		t.Fatalf("spend limit: %d", rr.Code)
	}
}

func TestFreeModelWithZeroBalance(t *testing.T) {
	e := newEnv(t, 0, func(s *store.Settings) { s.DefaultRequestFee = 0 })
	defer e.cleanup()
	e.ledger.balance = 0
	rr := e.do(t, "POST", "/v1/chat/completions", `{"model":"modelrouter/free","messages":[{"role":"user","content":"hi"}]}`, nil)
	if rr.Code != 200 {
		t.Fatalf("free model should work without balance: %d %s", rr.Code, rr.Body)
	}
	if e.ledger.last(t).Cost != 0 {
		t.Fatal("free model must not be charged")
	}
}

func TestRateLimit(t *testing.T) {
	e := newEnv(t, 0, func(s *store.Settings) { s.DefaultRPM = 2 })
	defer e.cleanup()
	body := `{"model":"modelrouter/fast","messages":[{"role":"user","content":"hi"}]}`
	for i := 0; i < 2; i++ {
		if rr := e.do(t, "POST", "/v1/chat/completions", body, nil); rr.Code != 200 {
			t.Fatalf("request %d: %d", i, rr.Code)
		}
	}
	rr := e.do(t, "POST", "/v1/chat/completions", body, nil)
	if rr.Code != 429 || rr.Header().Get("Retry-After") == "" {
		t.Fatalf("expected 429 with Retry-After, got %d", rr.Code)
	}
}

func TestRetryAndFallback(t *testing.T) {
	e := newEnv(t, 1, nil) // first upstream call fails with 503
	defer e.cleanup()
	rr := e.do(t, "POST", "/v1/chat/completions", `{"model":"fallback-model","messages":[{"role":"user","content":"hi"}]}`, nil)
	if rr.Code != 200 {
		t.Fatalf("fallback should succeed: %d %s", rr.Code, rr.Body)
	}
	if e.orRec.bodies[0]["model"] != "a/primary" || e.orRec.bodies[1]["model"] != "b/secondary" {
		t.Fatalf("fallback order wrong: %v / %v", e.orRec.bodies[0]["model"], e.orRec.bodies[1]["model"])
	}
	if rec := e.ledger.last(t); rec.UpstreamID != "b/secondary" {
		t.Fatalf("usage should record served upstream: %+v", rec)
	}
}

func TestUpstreamErrorSanitized(t *testing.T) {
	e := newEnv(t, 100, nil) // always 503
	defer e.cleanup()
	rr := e.do(t, "POST", "/v1/chat/completions", `{"model":"modelrouter/fast","messages":[{"role":"user","content":"hi"}]}`, nil)
	if rr.Code != 502 {
		t.Fatalf("status %d", rr.Code)
	}
	if strings.Contains(strings.ToLower(rr.Body.String()), "openrouter") {
		t.Fatalf("error leaks vendor: %s", rr.Body)
	}
	if rec := e.ledger.last(t); rec.Cost != 0 {
		t.Fatal("failed requests must not be charged")
	}
}

func TestModelsEndpoint(t *testing.T) {
	e := newEnv(t, 0, nil)
	defer e.cleanup()
	rr := e.do(t, "GET", "/v1/models", "", nil)
	out := rr.Body.String()
	if rr.Code != 200 || !strings.Contains(out, `"id":"modelrouter/fast"`) || strings.Contains(out, "openai/gpt-4o") || strings.Contains(out, `"disabled"`) {
		t.Fatalf("models: %s", out)
	}
	if !strings.Contains(out, `"prompt":"0.000002"`) {
		t.Fatalf("per-token price wrong: %s", out)
	}
	rr = e.do(t, "GET", "/v1/models", "", map[string]string{"Anthropic-Version": "2023-06-01"})
	if !strings.Contains(rr.Body.String(), `"display_name":"Fast"`) {
		t.Fatalf("anthropic models: %s", rr.Body)
	}
	rr = e.do(t, "GET", "/v1/models/modelrouter/fast", "", nil)
	if rr.Code != 200 {
		t.Fatalf("get model: %d", rr.Code)
	}
}

func TestCountTokens(t *testing.T) {
	e := newEnv(t, 0, nil)
	defer e.cleanup()
	rr := e.do(t, "POST", "/v1/messages/count_tokens", `{"model":"x","messages":[{"role":"user","content":"hello world"}]}`, nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "input_tokens") {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
}

func TestBodyTooLarge(t *testing.T) {
	e := newEnv(t, 0, nil)
	defer e.cleanup()
	e.h.MaxBody = 10
	rr := e.do(t, "POST", "/v1/chat/completions", `{"model":"modelrouter/fast","messages":[]}`, nil)
	if rr.Code != 413 {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestPerToken(t *testing.T) {
	cases := map[int64]string{0: "0", 3 * billing.NanoPerUSD: "0.000003", 37_500_000: "0.0000000375", 1_000_000 * billing.NanoPerUSD: "1"}
	for in, want := range cases {
		if got := perToken(in); got != want {
			t.Errorf("perToken(%d) = %s want %s", in, got, want)
		}
	}
}

func BenchmarkRewriteChunk(b *testing.B) {
	chunk := []byte(`{"id":"gen-123","provider":"OpenAI","model":"openai/gpt-4o","object":"chat.completion.chunk","created":1,"choices":[{"index":0,"delta":{"content":"Hello world"},"finish_reason":null}]}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = rewriteOpenAIChunk(chunk, "modelrouter/fast", "chatcmpl-1", 1, false)
	}
}

func BenchmarkChatCompletion(b *testing.B) {
	t := &testing.T{}
	e := newEnv(t, 0, func(s *store.Settings) { s.DefaultRPM = 0 })
	defer e.cleanup()
	body := []byte(`{"model":"modelrouter/fast","messages":[{"role":"user","content":"hi"}]}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+testKey)
		rr := httptest.NewRecorder()
		e.mux.ServeHTTP(rr, req)
		if rr.Code != 200 {
			b.Fatal(rr.Code)
		}
	}
}
