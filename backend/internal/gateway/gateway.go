// Package gateway serves the OpenAI- and Anthropic-compatible inference API.
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/billing"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/catalog"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/httpx"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/keyauth"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/ratelimit"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/translate"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/upstream"
)

const (
	FormatOpenAI    = "openai"
	FormatAnthropic = "anthropic"
)

type Ledger interface {
	Available(ctx context.Context, userID string) (int64, error)
	PendingKeySpend(userID, keyID string) int64
	Record(rec store.UsageRecord)
}

type KeyResolver interface {
	Lookup(ctx context.Context, rawKey string) (*store.KeyAuth, error)
}

type Handler struct {
	Keys     KeyResolver
	Catalog  *catalog.Catalog
	Ledger   Ledger
	Upstream *upstream.Client
	Limiter  *ratelimit.Limiter
	Log      *slog.Logger
	MaxBody  int64
	// DefaultAnthropicMaxTokens is used when an OpenAI request without max_tokens
	// is sent to a native Anthropic provider (which requires it).
	DefaultAnthropicMaxTokens int
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/chat/completions", h.chatCompletions)
	mux.HandleFunc("POST /api/v1/chat/completions", h.chatCompletions)
	mux.HandleFunc("POST /v1/messages", h.messages)
	mux.HandleFunc("POST /api/v1/messages", h.messages)
	mux.HandleFunc("POST /v1/messages/count_tokens", h.countTokens)
	mux.HandleFunc("GET /v1/models", h.listModels)
	mux.HandleFunc("GET /api/v1/models", h.listModels)
	mux.HandleFunc("GET /v1/models/{id...}", h.getModel)
	mux.HandleFunc("GET /v1/key", h.keyInfo)
}

// apiError is an error with an HTTP status destined for the client.
type apiError struct {
	status int
	msg    string
}

func (e *apiError) Error() string { return e.msg }

func errf(status int, msg string) *apiError { return &apiError{status: status, msg: msg} }

func writeErr(w http.ResponseWriter, format string, status int, msg string) {
	if status == http.StatusTooManyRequests && w.Header().Get("Retry-After") == "" {
		w.Header().Set("Retry-After", "1")
	}
	if format == FormatAnthropic {
		httpx.AnthropicError(w, status, httpx.AnthropicErrorType(status), msg)
		return
	}
	httpx.OpenAIError(w, status, httpx.OpenAIErrorType(status), msg)
}

// ---------------------------------------------------------------------------
// Request admission (auth, model, limits, balance)
// ---------------------------------------------------------------------------

type admitted struct {
	key   *store.KeyAuth
	entry *catalog.Entry
	snap  *catalog.Snapshot
}

func (h *Handler) authenticate(r *http.Request) (*store.KeyAuth, *apiError) {
	raw := keyauth.FromHeaders(r.Header.Get("Authorization"), r.Header.Get("X-Api-Key"))
	if raw == "" {
		return nil, errf(401, "Missing API key. Pass it as 'Authorization: Bearer <key>' or 'x-api-key: <key>'.")
	}
	k, err := h.Keys.Lookup(r.Context(), raw)
	if errors.Is(err, keyauth.ErrInvalidKey) {
		return nil, errf(401, "Invalid API key.")
	}
	if err != nil {
		h.Log.Error("key lookup failed", "err", err)
		return nil, errf(503, "Service temporarily unavailable, please retry.")
	}
	if k.UserBanned {
		return nil, errf(403, "This account has been suspended.")
	}
	if k.Disabled {
		return nil, errf(401, "This API key is disabled.")
	}
	return k, nil
}

func (h *Handler) admit(r *http.Request, model string, estInputTokens int) (*admitted, *apiError) {
	key, aerr := h.authenticate(r)
	if aerr != nil {
		return nil, aerr
	}
	snap := h.Catalog.Snapshot()
	if snap.Settings.MaintenanceMode {
		return nil, errf(503, "ModelRouter is undergoing maintenance. Please retry shortly.")
	}
	if model == "" {
		return nil, errf(400, "'model' is required.")
	}
	entry, ok := snap.Lookup(model)
	if !ok {
		return nil, errf(404, "Model '"+model+"' does not exist or is not available. See GET /v1/models.")
	}

	// Rate limits: per key, plus a tighter per-user bucket for free models.
	rpm := snap.Settings.DefaultRPM
	if key.RPMLimit != nil {
		rpm = *key.RPMLimit
	}
	if ok, wait := h.Limiter.Allow("k:"+key.KeyID, rpm); !ok {
		return nil, rateLimited(wait)
	}
	if entry.IsFree() && snap.Settings.FreeModelRPM > 0 {
		if ok, wait := h.Limiter.Allow("f:"+key.UserID, snap.Settings.FreeModelRPM); !ok {
			return nil, rateLimited(wait)
		}
	}

	if !entry.Price.IsFree() {
		avail, err := h.Ledger.Available(r.Context(), key.UserID)
		if err != nil {
			h.Log.Error("balance lookup failed", "err", err)
			return nil, errf(503, "Service temporarily unavailable, please retry.")
		}
		need := entry.Price.Cost(estInputTokens, 0)
		if avail <= 0 || avail < need {
			return nil, errf(402, "Insufficient balance. Top up your account at the dashboard to continue.")
		}
		if key.SpendLimit != nil && key.KeySpent+h.Ledger.PendingKeySpend(key.UserID, key.KeyID) >= *key.SpendLimit {
			return nil, errf(402, "This API key has reached its spend limit.")
		}
	}
	return &admitted{key: key, entry: entry, snap: snap}, nil
}

func rateLimited(wait time.Duration) *apiError {
	secs := int(math.Ceil(wait.Seconds()))
	if secs < 1 {
		secs = 1
	}
	return &apiError{status: 429, msg: "Rate limit exceeded. Retry after " + strconv.Itoa(secs) + "s."}
}

// capTokens clamps a requested max token value to the model's cap.
func capTokens(requested, limit int) int {
	if limit <= 0 {
		return requested
	}
	if requested <= 0 || requested > limit {
		return limit
	}
	return requested
}

// ---------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------

// exchange describes one client request being proxied.
type exchange struct {
	format       string // inbound format
	adm          *admitted
	stream       bool
	wantUsage    bool // OpenAI clients that asked for stream usage
	reqBytes     int
	requestID    string
	buildBody    func(upstreamID string) ([]byte, error)
	start        time.Time
	ttft         time.Duration
	outputBytes  int
	usage        translate.Usage
	upstreamUsed string
}

func (h *Handler) execute(w http.ResponseWriter, r *http.Request, ex *exchange) {
	entry := ex.adm.entry
	p := entry.Provider
	candidates := append([]string{entry.Model.UpstreamID}, entry.Model.FallbackUpstream...)
	if len(candidates) == 1 {
		candidates = append(candidates, entry.Model.UpstreamID) // one retry on transient errors
	}

	var lastStatus int
	var lastMsg string
	for attempt, upstreamID := range candidates {
		if attempt > 0 {
			select {
			case <-r.Context().Done():
				h.record(ex, 499, "client closed request")
				return
			case <-time.After(time.Duration(attempt) * 150 * time.Millisecond):
			}
		}
		body, err := ex.buildBody(upstreamID)
		if err != nil {
			writeErr(w, ex.format, 400, err.Error())
			return
		}
		resp, err := h.Upstream.Chat(r.Context(), p, body, ex.stream)
		if err != nil {
			if r.Context().Err() != nil {
				h.record(ex, 499, "client closed request")
				return
			}
			h.Log.Warn("upstream request failed", "provider", p.Name, "model", upstreamID, "err", err)
			lastStatus, lastMsg = 502, "Upstream connection failed."
			continue
		}
		if resp.StatusCode != http.StatusOK {
			errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			lastStatus, lastMsg = mapUpstreamError(resp.StatusCode, errBody)
			if resp.StatusCode == 401 || resp.StatusCode == 402 || resp.StatusCode == 403 {
				h.Log.Error("upstream rejected platform credentials or credit", "provider", p.Name, "status", resp.StatusCode,
					"body", upstream.Sanitize(string(errBody)))
			}
			if upstream.Retryable(resp.StatusCode) || resp.StatusCode == 404 || resp.StatusCode == 402 {
				continue
			}
			break
		}
		ex.upstreamUsed = upstreamID
		if ex.stream {
			h.relayStream(w, r, ex, resp)
		} else {
			h.relayJSON(w, ex, resp)
		}
		return
	}
	if lastStatus == 0 {
		lastStatus, lastMsg = 502, "Upstream unavailable."
	}
	h.record(ex, lastStatus, lastMsg)
	writeErr(w, ex.format, lastStatus, lastMsg)
}

func mapUpstreamError(status int, body []byte) (int, string) {
	msg := upstream.ErrorMessage(body)
	switch {
	case status == 400 || status == 413 || status == 422:
		return 400, msg
	case status == 401 || status == 403:
		return 502, "The model provider rejected the request. Please try again later."
	case status == 402:
		return 503, "This model is temporarily unavailable. Please try again later."
	case status == 404:
		return 503, "This model is temporarily unavailable."
	case status == 429:
		return 429, "The model is rate limited upstream. Please retry shortly."
	case status == 408 || status == 504:
		return 504, "The model timed out."
	default:
		return 502, "The model provider returned an error: " + msg
	}
}

// relayJSON handles a non-streaming upstream response.
func (h *Handler) relayJSON(w http.ResponseWriter, ex *exchange, resp *http.Response) {
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
	ex.ttft = time.Since(ex.start)
	if err != nil {
		h.record(ex, 502, "read upstream: "+err.Error())
		writeErr(w, ex.format, 502, "Failed to read the upstream response.")
		return
	}
	slug := ex.adm.entry.Model.Slug
	var out []byte
	switch {
	case ex.format == FormatOpenAI && ex.adm.entry.Provider.Kind == upstream.KindOpenAI:
		out, ex.usage, err = rewriteOpenAIResponse(body, slug, ex.requestID)
	case ex.format == FormatAnthropic && ex.adm.entry.Provider.Kind == upstream.KindAnthropic:
		out, ex.usage, err = rewriteAnthropicResponse(body, slug)
	case ex.format == FormatAnthropic:
		var r translate.OAIResponse
		if err = json.Unmarshal(body, &r); err == nil {
			ex.usage = translate.UsageFromOpenAI(r.Usage)
			out, err = json.Marshal(translate.OpenAIResponseToAnthropic(&r, slug, "msg_"+ex.requestID))
		}
	default: // OpenAI client, Anthropic provider
		var r translate.AnthResponse
		if err = json.Unmarshal(body, &r); err == nil {
			ex.usage = translate.UsageFromAnthropic(r.Usage)
			out, err = json.Marshal(translate.AnthropicResponseToOpenAI(&r, slug, "chatcmpl-"+ex.requestID, time.Now().Unix()))
		}
	}
	if err != nil {
		h.record(ex, 502, "decode upstream: "+err.Error())
		writeErr(w, ex.format, 502, "The model returned an invalid response.")
		return
	}
	ex.outputBytes = len(out)
	h.record(ex, 200, "")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(out)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

// flushWriter flushes the response after every write so tokens reach the
// client immediately.
type flushWriter struct {
	w http.ResponseWriter
	f http.Flusher
	n int
}

func (f *flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	f.n += n
	if f.f != nil {
		f.f.Flush()
	}
	return n, err
}

func (h *Handler) relayStream(w http.ResponseWriter, r *http.Request, ex *exchange, resp *http.Response) {
	defer resp.Body.Close()
	hdr := w.Header()
	hdr.Set("Content-Type", "text/event-stream")
	hdr.Set("Cache-Control", "no-cache")
	hdr.Set("Connection", "keep-alive")
	hdr.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	fw := &flushWriter{w: w, f: fl}

	slug := ex.adm.entry.Model.Slug
	kind := ex.adm.entry.Provider.Kind
	sse := translate.NewSSEReader(resp.Body)

	var oaiToAnth *translate.OAIToAnthStream
	var anthToOAI *translate.AnthToOAIStream
	switch {
	case ex.format == FormatAnthropic && kind == upstream.KindOpenAI:
		oaiToAnth = translate.NewOAIToAnthStream(fw, slug, "msg_"+ex.requestID)
	case ex.format == FormatOpenAI && kind == upstream.KindAnthropic:
		anthToOAI = translate.NewAnthToOAIStream(fw, slug, "chatcmpl-"+ex.requestID, time.Now().Unix(), ex.wantUsage)
	}

	created := time.Now().Unix()
	status, errMsg := 200, ""
	gotFirst := false
	done := false
	for !done {
		ev, err := sse.Next()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				if r.Context().Err() != nil {
					status, errMsg = 499, "client closed request"
				} else {
					status, errMsg = 502, "upstream stream interrupted"
				}
			}
			break
		}
		if ev.Comment {
			_, _ = io.WriteString(fw, ": keep-alive\n\n")
			continue
		}
		if !gotFirst {
			gotFirst = true
			ex.ttft = time.Since(ex.start)
		}
		switch {
		case oaiToAnth != nil:
			if bytes.Equal(ev.Data, []byte("[DONE]")) {
				done = true
				break
			}
			ex.outputBytes += len(ev.Data)
			err = oaiToAnth.Chunk(ev.Data)
		case anthToOAI != nil:
			ex.outputBytes += len(ev.Data)
			err = anthToOAI.Event(ev.Event, ev.Data)
			if ev.Event == "message_stop" {
				done = true
			}
		case kind == upstream.KindOpenAI: // OpenAI passthrough with rewriting
			if bytes.Equal(ev.Data, []byte("[DONE]")) {
				err = translate.WriteSSE(fw, "", ev.Data)
				done = true
				break
			}
			ex.outputBytes += len(ev.Data)
			var out []byte
			var u *translate.OAIUsage
			var keep bool
			out, u, keep, err = rewriteOpenAIChunk(ev.Data, slug, "chatcmpl-"+ex.requestID, created, ex.wantUsage)
			if u != nil {
				ex.usage = translate.UsageFromOpenAI(u)
			}
			if err == nil && keep {
				err = translate.WriteSSE(fw, "", out)
			}
		default: // Anthropic passthrough
			ex.outputBytes += len(ev.Data)
			var out []byte
			out, err = rewriteAnthropicEvent(ev.Event, ev.Data, slug, &ex.usage)
			if err == nil {
				err = translate.WriteSSE(fw, ev.Event, out)
			}
			if ev.Event == "message_stop" {
				done = true
			}
		}
		if err != nil {
			status, errMsg = 499, "write to client: "+err.Error()
			break
		}
	}

	if oaiToAnth != nil {
		if u := oaiToAnth.Usage(); u.Known() {
			ex.usage = u
		}
		if status == 200 {
			_ = oaiToAnth.Finish()
		} else if status == 502 {
			_ = oaiToAnth.Error("The upstream stream was interrupted.")
		}
	}
	if anthToOAI != nil {
		if u := anthToOAI.Usage(); u.Known() {
			ex.usage = u
		}
		if status == 200 {
			_ = anthToOAI.Finish()
		} else if status == 502 {
			_ = anthToOAI.Error("The upstream stream was interrupted.")
		}
	}
	h.record(ex, status, errMsg)
}

// record computes the charge for an exchange and hands it to the ledger.
func (h *Handler) record(ex *exchange, status int, errMsg string) {
	e := ex.adm.entry
	u := ex.usage
	charge := status == 200 || (status == 499 && ex.outputBytes > 0) || (status == 502 && ex.outputBytes > 0)
	if charge && !u.Known() {
		// Upstream did not report usage (e.g. the client disconnected): estimate.
		u.InputTokens = translate.EstimateTokens(ex.reqBytes)
		u.OutputTokens = translate.EstimateTokens(ex.outputBytes / 3)
	}
	var cost, upstreamCost int64
	if charge {
		cost = e.Price.Cost(u.InputTokens, u.OutputTokens)
		upstreamCost = e.UpstreamPrice.Cost(u.InputTokens, u.OutputTokens)
		if u.UpstreamCostUSD != nil && *u.UpstreamCostUSD > 0 {
			upstreamCost = int64(math.Ceil(*u.UpstreamCostUSD * float64(billing.NanoPerUSD)))
		}
	}
	upID := ex.upstreamUsed
	if upID == "" {
		upID = e.Model.UpstreamID
	}
	h.Ledger.Record(store.UsageRecord{
		RequestID:    ex.requestID,
		UserID:       ex.adm.key.UserID,
		APIKeyID:     ex.adm.key.KeyID,
		ModelSlug:    e.Model.Slug,
		UpstreamID:   upID,
		ProviderID:   e.Provider.ID,
		Endpoint:     ex.format,
		Stream:       ex.stream,
		Status:       status,
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		CachedTokens: u.CachedTokens,
		Cost:         cost,
		UpstreamCost: upstreamCost,
		LatencyMS:    int(time.Since(ex.start).Milliseconds()),
		TTFTMS:       int(ex.ttft.Milliseconds()),
		Error:        errMsg,
		CreatedAt:    ex.start,
	})
}
