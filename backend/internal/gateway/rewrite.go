package gateway

import (
	"encoding/json"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/translate"
)

// Keys that reveal the upstream vendor or its internal costs.
var hiddenResponseKeys = []string{"provider", "native_finish_reason"}
var hiddenUsageKeys = []string{"cost", "cost_details", "is_byok"}

func scrubUsage(raw json.RawMessage) (json.RawMessage, *translate.OAIUsage) {
	if len(raw) == 0 || string(raw) == "null" {
		return raw, nil
	}
	var u translate.OAIUsage
	_ = json.Unmarshal(raw, &u)
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return raw, &u
	}
	for _, k := range hiddenUsageKeys {
		delete(m, k)
	}
	out, _ := json.Marshal(m)
	return out, &u
}

func scrubChoices(raw json.RawMessage) json.RawMessage {
	var choices []map[string]json.RawMessage
	if json.Unmarshal(raw, &choices) != nil {
		return raw
	}
	for _, c := range choices {
		for _, k := range hiddenResponseKeys {
			delete(c, k)
		}
	}
	out, _ := json.Marshal(choices)
	return out
}

// rewriteOpenAIResponse replaces model/id and strips upstream-identifying data.
func rewriteOpenAIResponse(body []byte, model, reqID string) ([]byte, translate.Usage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, translate.Usage{}, err
	}
	for _, k := range hiddenResponseKeys {
		delete(m, k)
	}
	m["model"], _ = json.Marshal(model)
	m["id"], _ = json.Marshal("chatcmpl-" + reqID)
	if c, ok := m["choices"]; ok {
		m["choices"] = scrubChoices(c)
	}
	var usage translate.Usage
	if raw, ok := m["usage"]; ok {
		var u *translate.OAIUsage
		m["usage"], u = scrubUsage(raw)
		usage = translate.UsageFromOpenAI(u)
	}
	out, err := json.Marshal(m)
	return out, usage, err
}

// rewriteOpenAIChunk rewrites one streaming chunk. keep=false means the chunk
// should not be forwarded (a usage-only chunk the client did not ask for).
func rewriteOpenAIChunk(data []byte, model, id string, created int64, wantUsage bool) (out []byte, usage *translate.OAIUsage, keep bool, err error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil, false, nil // drop malformed payloads
	}
	for _, k := range hiddenResponseKeys {
		delete(m, k)
	}
	m["model"], _ = json.Marshal(model)
	m["id"], _ = json.Marshal(id)
	m["created"], _ = json.Marshal(created)
	if c, ok := m["choices"]; ok {
		m["choices"] = scrubChoices(c)
	}
	if raw, ok := m["usage"]; ok && string(raw) != "null" {
		m["usage"], usage = scrubUsage(raw)
		if !wantUsage {
			delete(m, "usage")
			var choices []json.RawMessage
			_ = json.Unmarshal(m["choices"], &choices)
			if len(choices) == 0 {
				return nil, usage, false, nil
			}
		}
	}
	out, err = json.Marshal(m)
	return out, usage, true, err
}

func rewriteAnthropicResponse(body []byte, model string) ([]byte, translate.Usage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, translate.Usage{}, err
	}
	m["model"], _ = json.Marshal(model)
	var u translate.AnthUsage
	_ = json.Unmarshal(m["usage"], &u)
	out, err := json.Marshal(m)
	return out, translate.UsageFromAnthropic(u), err
}

// rewriteAnthropicEvent rewrites the model in message_start and tracks usage.
func rewriteAnthropicEvent(event string, data []byte, model string, usage *translate.Usage) ([]byte, error) {
	switch event {
	case "message_start":
		var ev struct {
			Type    string                     `json:"type"`
			Message map[string]json.RawMessage `json:"message"`
		}
		if err := json.Unmarshal(data, &ev); err != nil || ev.Message == nil {
			return data, nil
		}
		var u translate.AnthUsage
		_ = json.Unmarshal(ev.Message["usage"], &u)
		*usage = translate.UsageFromAnthropic(u)
		ev.Message["model"], _ = json.Marshal(model)
		return json.Marshal(ev)
	case "message_delta":
		var ev struct {
			Usage *translate.AnthUsage `json:"usage"`
		}
		if json.Unmarshal(data, &ev) == nil && ev.Usage != nil {
			usage.OutputTokens = ev.Usage.OutputTokens
			if ev.Usage.InputTokens > 0 {
				in := translate.UsageFromAnthropic(*ev.Usage)
				usage.InputTokens = in.InputTokens
				usage.CachedTokens = in.CachedTokens
			}
		}
	}
	return data, nil
}
