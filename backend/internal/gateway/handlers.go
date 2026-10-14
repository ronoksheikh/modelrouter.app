package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/billing"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/catalog"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/httpx"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/translate"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/upstream"
)

// Fields that would expose or alter upstream routing; never forwarded.
var strippedFields = []string{"models", "route", "provider", "transforms", "plugins", "preset"}

func (h *Handler) readBody(w http.ResponseWriter, r *http.Request, format string) ([]byte, bool) {
	body, err := httpx.ReadBody(r, h.MaxBody)
	if errors.Is(err, httpx.ErrBodyTooLarge) {
		writeErr(w, format, 413, "Request body too large.")
		return nil, false
	}
	if err != nil {
		writeErr(w, format, 400, "Could not read request body.")
		return nil, false
	}
	return body, true
}

// ---------------------------------------------------------------------------
// POST /v1/chat/completions
// ---------------------------------------------------------------------------

func (h *Handler) chatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	body, ok := h.readBody(w, r, FormatOpenAI)
	if !ok {
		return
	}
	var head struct {
		Model         string `json:"model"`
		Stream        bool   `json:"stream"`
		StreamOptions *struct {
			IncludeUsage bool `json:"include_usage"`
		} `json:"stream_options"`
	}
	if err := json.Unmarshal(body, &head); err != nil {
		writeErr(w, FormatOpenAI, 400, "Invalid JSON body: "+err.Error())
		return
	}
	adm, aerr := h.admit(r, head.Model, translate.EstimateTokens(len(body)))
	if aerr != nil {
		writeErr(w, FormatOpenAI, aerr.status, aerr.msg)
		return
	}
	ex := &exchange{
		format: FormatOpenAI, adm: adm, stream: head.Stream, reqBytes: len(body),
		wantUsage: head.StreamOptions != nil && head.StreamOptions.IncludeUsage,
		requestID: requestID(r), start: start,
	}
	limit := adm.entry.Model.MaxOutputTokens
	provider := adm.entry.Provider

	if provider.Kind == upstream.KindAnthropic {
		var req translate.OAIRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeErr(w, FormatOpenAI, 400, "Invalid request: "+err.Error())
			return
		}
		def := capTokens(h.DefaultAnthropicMaxTokens, limit)
		ar, err := translate.OpenAIToAnthropic(&req, def)
		if err != nil {
			writeErr(w, FormatOpenAI, 400, err.Error())
			return
		}
		ar.MaxTokens = capTokens(ar.MaxTokens, limit)
		ex.buildBody = func(upstreamID string) ([]byte, error) {
			ar.Model = upstreamID
			return json.Marshal(ar)
		}
	} else {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(body, &m); err != nil {
			writeErr(w, FormatOpenAI, 400, "Invalid JSON body.")
			return
		}
		if _, ok := m["messages"]; !ok {
			writeErr(w, FormatOpenAI, 400, "'messages' is required.")
			return
		}
		for _, f := range strippedFields {
			delete(m, f)
		}
		applyTokenCap(m, limit)
		if head.Stream {
			m["stream_options"] = json.RawMessage(`{"include_usage":true}`)
		}
		if provider.IsOpenRouter {
			m["usage"] = json.RawMessage(`{"include":true}`)
		}
		ex.buildBody = func(upstreamID string) ([]byte, error) {
			m["model"], _ = json.Marshal(upstreamID)
			return json.Marshal(m)
		}
	}
	h.execute(w, r, ex)
}

// applyTokenCap clamps max_tokens / max_completion_tokens to the model cap.
func applyTokenCap(m map[string]json.RawMessage, limit int) {
	if limit <= 0 {
		return
	}
	found := false
	for _, k := range []string{"max_tokens", "max_completion_tokens"} {
		raw, ok := m[k]
		if !ok || string(raw) == "null" {
			continue
		}
		found = true
		var v int
		if json.Unmarshal(raw, &v) != nil || v <= 0 || v > limit {
			m[k], _ = json.Marshal(limit)
		}
	}
	if !found {
		m["max_tokens"], _ = json.Marshal(limit)
	}
}

// ---------------------------------------------------------------------------
// POST /v1/messages
// ---------------------------------------------------------------------------

func (h *Handler) messages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	body, ok := h.readBody(w, r, FormatAnthropic)
	if !ok {
		return
	}
	var req translate.AnthRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, FormatAnthropic, 400, "Invalid JSON body: "+err.Error())
		return
	}
	adm, aerr := h.admit(r, req.Model, translate.EstimateTokens(len(body)))
	if aerr != nil {
		writeErr(w, FormatAnthropic, aerr.status, aerr.msg)
		return
	}
	if len(req.Messages) == 0 {
		writeErr(w, FormatAnthropic, 400, "messages: at least one message is required")
		return
	}
	if req.MaxTokens <= 0 {
		writeErr(w, FormatAnthropic, 400, "max_tokens: field required")
		return
	}
	ex := &exchange{format: FormatAnthropic, adm: adm, stream: req.Stream, reqBytes: len(body), requestID: requestID(r), start: start}
	limit := adm.entry.Model.MaxOutputTokens
	provider := adm.entry.Provider

	if provider.Kind == upstream.KindAnthropic {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(body, &m); err != nil {
			writeErr(w, FormatAnthropic, 400, "Invalid JSON body.")
			return
		}
		if limit > 0 && req.MaxTokens > limit {
			m["max_tokens"], _ = json.Marshal(limit)
		}
		ex.buildBody = func(upstreamID string) ([]byte, error) {
			m["model"], _ = json.Marshal(upstreamID)
			return json.Marshal(m)
		}
	} else {
		oreq, err := translate.AnthropicToOpenAI(&req)
		if err != nil {
			writeErr(w, FormatAnthropic, 400, err.Error())
			return
		}
		mt := capTokens(req.MaxTokens, limit)
		oreq.MaxTokens = &mt
		if req.Stream {
			oreq.StreamOptions = &translate.StreamOptions{IncludeUsage: true}
		}
		ex.buildBody = func(upstreamID string) ([]byte, error) {
			oreq.Model = upstreamID
			b, err := json.Marshal(oreq)
			if err != nil || !provider.IsOpenRouter {
				return b, err
			}
			// Ask OpenRouter for usage accounting (exact upstream cost).
			return append(b[:len(b)-1], []byte(`,"usage":{"include":true}}`)...), nil
		}
	}
	h.execute(w, r, ex)
}

// POST /v1/messages/count_tokens returns an estimate.
func (h *Handler) countTokens(w http.ResponseWriter, r *http.Request) {
	if _, aerr := h.authenticate(r); aerr != nil {
		writeErr(w, FormatAnthropic, aerr.status, aerr.msg)
		return
	}
	body, ok := h.readBody(w, r, FormatAnthropic)
	if !ok {
		return
	}
	var req struct {
		Messages json.RawMessage `json:"messages"`
		System   json.RawMessage `json:"system"`
		Tools    json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, FormatAnthropic, 400, "Invalid JSON body.")
		return
	}
	n := translate.EstimateTokens(len(req.Messages) + len(req.System) + len(req.Tools))
	httpx.JSON(w, 200, map[string]int{"input_tokens": n})
}

// ---------------------------------------------------------------------------
// Models & key info
// ---------------------------------------------------------------------------

type publicPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Request    string `json:"request"`
}

func pricingOf(e *catalog.Entry) publicPricing {
	// Per-token USD strings, like the OpenAI/OpenRouter-style catalogs.
	return publicPricing{
		Prompt:     perToken(e.Price.InputPerM),
		Completion: perToken(e.Price.OutputPerM),
		Request:    billing.FormatUSD(e.Price.RequestFee),
	}
}

func perToken(perM int64) string {
	// perM is nano-USD per 1M tokens; per token USD = perM / 1e15.
	if perM == 0 {
		return "0"
	}
	s := []byte(strings.Repeat("0", 16) + itoa(perM))
	intPart := strings.TrimLeft(string(s[:len(s)-15]), "0")
	frac := strings.TrimRight(string(s[len(s)-15:]), "0")
	if intPart == "" {
		intPart = "0"
	}
	if frac == "" {
		return intPart
	}
	return intPart + "." + frac
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func modelJSON(e *catalog.Entry) map[string]any {
	m := e.Model
	return map[string]any{
		"id":                m.Slug,
		"object":            "model",
		"created":           m.CreatedAt.Unix(),
		"owned_by":          ownedBy(m.Author),
		"name":              m.Name,
		"description":       m.Description,
		"context_length":    m.ContextLength,
		"max_output_tokens": m.MaxOutputTokens,
		"modalities":        m.Modalities,
		"supports_tools":    m.SupportsTools,
		"pricing":           pricingOf(e),
	}
}

func ownedBy(author string) string {
	if author == "" {
		return "modelrouter"
	}
	return author
}

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request) {
	snap := h.Catalog.Snapshot()
	if r.Header.Get("Anthropic-Version") != "" {
		data := make([]map[string]any, 0, len(snap.Models))
		for _, e := range snap.Models {
			data = append(data, map[string]any{
				"type": "model", "id": e.Model.Slug, "display_name": e.Model.Name,
				"created_at": e.Model.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		resp := map[string]any{"data": data, "has_more": false, "first_id": nil, "last_id": nil}
		if len(data) > 0 {
			resp["first_id"] = data[0]["id"]
			resp["last_id"] = data[len(data)-1]["id"]
		}
		httpx.JSON(w, 200, resp)
		return
	}
	data := make([]map[string]any, 0, len(snap.Models))
	for _, e := range snap.Models {
		data = append(data, modelJSON(e))
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	httpx.JSON(w, 200, map[string]any{"object": "list", "data": data})
}

func (h *Handler) getModel(w http.ResponseWriter, r *http.Request) {
	e, ok := h.Catalog.Snapshot().Lookup(r.PathValue("id"))
	if !ok {
		writeErr(w, FormatOpenAI, 404, "Model not found.")
		return
	}
	httpx.JSON(w, 200, modelJSON(e))
}

func (h *Handler) keyInfo(w http.ResponseWriter, r *http.Request) {
	key, aerr := h.authenticate(r)
	if aerr != nil {
		writeErr(w, FormatOpenAI, aerr.status, aerr.msg)
		return
	}
	avail, err := h.Ledger.Available(r.Context(), key.UserID)
	if err != nil {
		writeErr(w, FormatOpenAI, 503, "Service temporarily unavailable.")
		return
	}
	spent := key.KeySpent + h.Ledger.PendingKeySpend(key.UserID, key.KeyID)
	var limit any
	if key.SpendLimit != nil {
		limit = billing.FormatUSD(*key.SpendLimit)
	}
	httpx.JSON(w, 200, map[string]any{"data": map[string]any{
		"balance": billing.FormatUSD(avail),
		"usage":   billing.FormatUSD(spent),
		"limit":   limit,
	}})
}

func requestID(r *http.Request) string {
	id := httpx.RequestID(r.Context())
	return strings.TrimPrefix(id, "req_")
}
