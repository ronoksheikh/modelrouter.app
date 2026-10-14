package translate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var nullJSON = json.RawMessage("null")

func rawString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// contentString returns the content when it is a plain JSON string.
func contentString(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '"' {
		return "", false
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

func isNull(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) == 0 || bytes.Equal(raw, nullJSON)
}

// ============================================================================
// Anthropic request -> OpenAI request
// ============================================================================

func AnthropicToOpenAI(r *AnthRequest) (*OAIRequest, error) {
	out := &OAIRequest{
		Model:       r.Model,
		Temperature: r.Temperature,
		TopP:        r.TopP,
		TopK:        r.TopK,
		Stream:      r.Stream,
	}
	if r.MaxTokens > 0 {
		mt := r.MaxTokens
		out.MaxTokens = &mt
	}
	if len(r.StopSequences) > 0 {
		b, _ := json.Marshal(r.StopSequences)
		out.Stop = b
	}
	if r.Metadata != nil && r.Metadata.UserID != "" {
		out.User = r.Metadata.UserID
	}
	if r.Thinking != nil && r.Thinking.Type == "enabled" && r.Thinking.BudgetTokens > 0 {
		out.Reasoning = json.RawMessage(fmt.Sprintf(`{"max_tokens":%d}`, r.Thinking.BudgetTokens))
	}

	if sys, err := systemText(r.System); err != nil {
		return nil, err
	} else if sys != "" {
		out.Messages = append(out.Messages, OAIMessage{Role: "system", Content: rawString(sys)})
	}

	for i, m := range r.Messages {
		msgs, err := anthMessageToOpenAI(m)
		if err != nil {
			return nil, fmt.Errorf("messages[%d]: %w", i, err)
		}
		out.Messages = append(out.Messages, msgs...)
	}

	for _, t := range r.Tools {
		// Server tools (web_search, bash, ...) have a type and no schema; skip them.
		if t.Type != "" && t.Type != "custom" && len(t.InputSchema) == 0 {
			continue
		}
		params := t.InputSchema
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out.Tools = append(out.Tools, OAITool{Type: "function", Function: OAIFunction{
			Name: t.Name, Description: t.Description, Parameters: params,
		}})
	}

	if tc := r.ToolChoice; tc != nil {
		switch tc.Type {
		case "auto":
			out.ToolChoice = rawString("auto")
		case "any":
			out.ToolChoice = rawString("required")
		case "none":
			out.ToolChoice = rawString("none")
		case "tool":
			b, _ := json.Marshal(map[string]any{"type": "function", "function": map[string]string{"name": tc.Name}})
			out.ToolChoice = b
		}
		if tc.DisableParallelToolUse {
			f := false
			out.ParallelToolCalls = &f
		}
	}
	return out, nil
}

func systemText(raw json.RawMessage) (string, error) {
	if isNull(raw) {
		return "", nil
	}
	if s, ok := contentString(raw); ok {
		return s, nil
	}
	var blocks []AnthBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", errors.New("system: must be a string or an array of text blocks")
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

func anthBlocks(raw json.RawMessage) ([]AnthBlock, error) {
	if s, ok := contentString(raw); ok {
		return []AnthBlock{{Type: "text", Text: s}}, nil
	}
	if isNull(raw) {
		return nil, nil
	}
	var blocks []AnthBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, errors.New("content must be a string or an array of content blocks")
	}
	return blocks, nil
}

func anthMessageToOpenAI(m AnthMessage) ([]OAIMessage, error) {
	if s, ok := contentString(m.Content); ok {
		return []OAIMessage{{Role: m.Role, Content: rawString(s)}}, nil
	}
	blocks, err := anthBlocks(m.Content)
	if err != nil {
		return nil, err
	}

	switch m.Role {
	case "assistant":
		var text strings.Builder
		var calls []OAIToolCall
		for _, b := range blocks {
			switch b.Type {
			case "text":
				text.WriteString(b.Text)
			case "tool_use", "server_tool_use":
				args := "{}"
				if !isNull(b.Input) {
					args = string(b.Input)
				}
				calls = append(calls, OAIToolCall{ID: b.ID, Type: "function", Function: OAIFunctionCall{Name: b.Name, Arguments: args}})
			}
		}
		// Thinking blocks are dropped: their signatures are only meaningful to the
		// model that produced them.
		msg := OAIMessage{Role: "assistant", ToolCalls: calls}
		if text.Len() > 0 || len(calls) == 0 {
			msg.Content = rawString(text.String())
		} else {
			msg.Content = nullJSON
		}
		return []OAIMessage{msg}, nil

	case "user":
		var toolMsgs []OAIMessage
		var parts []OAIPart
		for _, b := range blocks {
			switch b.Type {
			case "text":
				parts = append(parts, OAIPart{Type: "text", Text: b.Text})
			case "image":
				if url := imageURL(b.Source); url != "" {
					parts = append(parts, OAIPart{Type: "image_url", ImageURL: &OAIImageURL{URL: url}})
				}
			case "document":
				if b.Source != nil && b.Source.Type == "text" {
					parts = append(parts, OAIPart{Type: "text", Text: b.Source.Data})
				}
			case "tool_result":
				txt := toolResultText(b.Content)
				if b.IsError && txt != "" {
					txt = "Error: " + txt
				}
				toolMsgs = append(toolMsgs, OAIMessage{Role: "tool", ToolCallID: b.ToolUseID, Content: rawString(txt)})
			}
		}
		out := toolMsgs
		if len(parts) > 0 {
			out = append(out, OAIMessage{Role: "user", Content: partsContent(parts)})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported role %q", m.Role)
	}
}

// partsContent collapses text-only parts into a plain string for maximum
// upstream compatibility.
func partsContent(parts []OAIPart) json.RawMessage {
	textOnly := true
	for _, p := range parts {
		if p.Type != "text" {
			textOnly = false
			break
		}
	}
	if textOnly {
		var sb strings.Builder
		for i, p := range parts {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(p.Text)
		}
		return rawString(sb.String())
	}
	b, _ := json.Marshal(parts)
	return b
}

func imageURL(src *AnthSource) string {
	if src == nil {
		return ""
	}
	switch src.Type {
	case "base64":
		return "data:" + src.MediaType + ";base64," + src.Data
	case "url":
		return src.URL
	}
	return ""
}

func toolResultText(raw json.RawMessage) string {
	if s, ok := contentString(raw); ok {
		return s
	}
	var blocks []AnthBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return string(raw)
	}
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "image":
			parts = append(parts, "[image]")
		}
	}
	return strings.Join(parts, "\n")
}

// ============================================================================
// OpenAI response -> Anthropic response
// ============================================================================

func MapFinishToStop(finish string) string {
	switch finish {
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	case "content_filter":
		return "refusal"
	default:
		return "end_turn"
	}
}

func MapStopToFinish(stop string) string {
	switch stop {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "refusal":
		return "content_filter"
	default:
		return "stop"
	}
}

func OpenAIResponseToAnthropic(r *OAIResponse, model, id string) *AnthResponse {
	out := &AnthResponse{ID: id, Type: "message", Role: "assistant", Model: model, Content: []AnthBlock{}}
	stop := "end_turn"
	if len(r.Choices) > 0 {
		c := r.Choices[0]
		if c.FinishReason != nil {
			stop = MapFinishToStop(*c.FinishReason)
		}
		if c.Message != nil {
			if s, ok := contentString(c.Message.Content); ok && s != "" {
				out.Content = append(out.Content, AnthBlock{Type: "text", Text: s})
			} else if !isNull(c.Message.Content) {
				var parts []OAIPart
				if json.Unmarshal(c.Message.Content, &parts) == nil {
					for _, p := range parts {
						if p.Type == "text" && p.Text != "" {
							out.Content = append(out.Content, AnthBlock{Type: "text", Text: p.Text})
						}
					}
				}
			}
			for _, tc := range c.Message.ToolCalls {
				out.Content = append(out.Content, AnthBlock{Type: "tool_use", ID: toolID(tc.ID), Name: tc.Function.Name, Input: parseArgs(tc.Function.Arguments)})
			}
			if len(c.Message.ToolCalls) > 0 {
				stop = "tool_use"
			}
		}
	}
	out.StopReason = &stop
	if r.Usage != nil {
		out.Usage = AnthUsage{InputTokens: r.Usage.PromptTokens, OutputTokens: r.Usage.CompletionTokens}
		if r.Usage.PromptTokensDetails != nil {
			out.Usage.CacheReadInputTokens = r.Usage.PromptTokensDetails.CachedTokens
		}
	}
	return out
}

func toolID(id string) string {
	if id == "" {
		return "toolu_" + randHex(12)
	}
	return id
}

func parseArgs(s string) json.RawMessage {
	s = strings.TrimSpace(s)
	if s == "" {
		return json.RawMessage("{}")
	}
	if json.Valid([]byte(s)) && s[0] == '{' {
		return json.RawMessage(s)
	}
	return json.RawMessage("{}")
}

func UsageFromOpenAI(u *OAIUsage) Usage {
	if u == nil {
		return Usage{}
	}
	out := Usage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens, UpstreamCostUSD: u.Cost}
	if u.PromptTokensDetails != nil {
		out.CachedTokens = u.PromptTokensDetails.CachedTokens
	}
	return out
}

func UsageFromAnthropic(u AnthUsage) Usage {
	return Usage{
		InputTokens:  u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens,
		OutputTokens: u.OutputTokens,
		CachedTokens: u.CacheReadInputTokens,
	}
}

// ============================================================================
// OpenAI request -> Anthropic request
// ============================================================================

func OpenAIToAnthropic(r *OAIRequest, defaultMaxTokens int) (*AnthRequest, error) {
	out := &AnthRequest{Model: r.Model, TopP: r.TopP, TopK: r.TopK, Stream: r.Stream}
	switch {
	case r.MaxCompletionTokens != nil && *r.MaxCompletionTokens > 0:
		out.MaxTokens = *r.MaxCompletionTokens
	case r.MaxTokens != nil && *r.MaxTokens > 0:
		out.MaxTokens = *r.MaxTokens
	default:
		out.MaxTokens = defaultMaxTokens
	}
	if r.Temperature != nil {
		t := *r.Temperature
		if t > 1 { // Anthropic accepts 0..1, OpenAI 0..2
			t = t / 2
		}
		out.Temperature = &t
	}
	if !isNull(r.Stop) {
		if s, ok := contentString(r.Stop); ok {
			out.StopSequences = []string{s}
		} else {
			_ = json.Unmarshal(r.Stop, &out.StopSequences)
		}
	}
	if r.User != "" {
		out.Metadata = &struct {
			UserID string `json:"user_id,omitempty"`
		}{UserID: r.User}
	}

	var system []string
	var msgs []AnthMessage
	appendBlocks := func(role string, blocks []AnthBlock) {
		if len(blocks) == 0 {
			return
		}
		// Merge consecutive messages with the same role (Anthropic requires alternation).
		if n := len(msgs); n > 0 && msgs[n-1].Role == role {
			var prev []AnthBlock
			_ = json.Unmarshal(msgs[n-1].Content, &prev)
			blocks = append(prev, blocks...)
			msgs = msgs[:n-1]
		}
		b, _ := json.Marshal(blocks)
		msgs = append(msgs, AnthMessage{Role: role, Content: b})
	}

	for i, m := range r.Messages {
		switch m.Role {
		case "system", "developer":
			system = append(system, oaiContentText(m.Content))
		case "user":
			blocks, err := oaiUserBlocks(m.Content)
			if err != nil {
				return nil, fmt.Errorf("messages[%d]: %w", i, err)
			}
			appendBlocks("user", blocks)
		case "assistant":
			var blocks []AnthBlock
			if txt := oaiContentText(m.Content); txt != "" {
				blocks = append(blocks, AnthBlock{Type: "text", Text: txt})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, AnthBlock{Type: "tool_use", ID: toolID(tc.ID), Name: tc.Function.Name, Input: parseArgs(tc.Function.Arguments)})
			}
			appendBlocks("assistant", blocks)
		case "tool", "function":
			appendBlocks("user", []AnthBlock{{Type: "tool_result", ToolUseID: m.ToolCallID, Content: rawString(oaiContentText(m.Content))}})
		default:
			return nil, fmt.Errorf("messages[%d]: unsupported role %q", i, m.Role)
		}
	}
	if len(system) > 0 {
		out.System = rawString(strings.Join(system, "\n\n"))
	}
	out.Messages = msgs

	for _, t := range r.Tools {
		if t.Type != "" && t.Type != "function" {
			continue
		}
		schema := t.Function.Parameters
		if isNull(schema) {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out.Tools = append(out.Tools, AnthTool{Name: t.Function.Name, Description: t.Function.Description, InputSchema: schema})
	}

	if !isNull(r.ToolChoice) {
		if s, ok := contentString(r.ToolChoice); ok {
			switch s {
			case "auto":
				out.ToolChoice = &AnthToolChoice{Type: "auto"}
			case "required":
				out.ToolChoice = &AnthToolChoice{Type: "any"}
			case "none":
				out.ToolChoice = &AnthToolChoice{Type: "none"}
			}
		} else {
			var tc struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			if json.Unmarshal(r.ToolChoice, &tc) == nil && tc.Function.Name != "" {
				out.ToolChoice = &AnthToolChoice{Type: "tool", Name: tc.Function.Name}
			}
		}
	}
	if r.ParallelToolCalls != nil && !*r.ParallelToolCalls && len(out.Tools) > 0 {
		if out.ToolChoice == nil {
			out.ToolChoice = &AnthToolChoice{Type: "auto"}
		}
		out.ToolChoice.DisableParallelToolUse = true
	}
	return out, nil
}

func oaiContentText(raw json.RawMessage) string {
	if s, ok := contentString(raw); ok {
		return s
	}
	var parts []OAIPart
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

func oaiUserBlocks(raw json.RawMessage) ([]AnthBlock, error) {
	if s, ok := contentString(raw); ok {
		return []AnthBlock{{Type: "text", Text: s}}, nil
	}
	if isNull(raw) {
		return nil, nil
	}
	var parts []OAIPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, errors.New("content must be a string or an array of content parts")
	}
	var out []AnthBlock
	for _, p := range parts {
		switch p.Type {
		case "text":
			out = append(out, AnthBlock{Type: "text", Text: p.Text})
		case "image_url":
			if p.ImageURL == nil {
				continue
			}
			out = append(out, AnthBlock{Type: "image", Source: sourceFromURL(p.ImageURL.URL)})
		}
	}
	return out, nil
}

func sourceFromURL(u string) *AnthSource {
	if strings.HasPrefix(u, "data:") {
		// data:<media>;base64,<data>
		rest := u[len("data:"):]
		if i := strings.Index(rest, ";base64,"); i >= 0 {
			return &AnthSource{Type: "base64", MediaType: rest[:i], Data: rest[i+len(";base64,"):]}
		}
	}
	return &AnthSource{Type: "url", URL: u}
}

// ============================================================================
// Anthropic response -> OpenAI response
// ============================================================================

func AnthropicResponseToOpenAI(r *AnthResponse, model, id string, created int64) *OAIResponse {
	var text strings.Builder
	var reasoning strings.Builder
	var calls []OAIToolCall
	for _, b := range r.Content {
		switch b.Type {
		case "text":
			text.WriteString(b.Text)
		case "thinking":
			reasoning.WriteString(b.Thinking)
		case "tool_use":
			args := "{}"
			if !isNull(b.Input) {
				args = string(b.Input)
			}
			calls = append(calls, OAIToolCall{ID: b.ID, Type: "function", Function: OAIFunctionCall{Name: b.Name, Arguments: args}})
		}
	}
	finish := "stop"
	if r.StopReason != nil {
		finish = MapStopToFinish(*r.StopReason)
	}
	msg := &OAIMessage{Role: "assistant", ToolCalls: calls, Reasoning: reasoning.String()}
	if text.Len() > 0 || len(calls) == 0 {
		msg.Content = rawString(text.String())
	} else {
		msg.Content = nullJSON
	}
	u := UsageFromAnthropic(r.Usage)
	usage := &OAIUsage{PromptTokens: u.InputTokens, CompletionTokens: u.OutputTokens, TotalTokens: u.InputTokens + u.OutputTokens}
	if u.CachedTokens > 0 {
		usage.PromptTokensDetails = &PromptTokenDetails{CachedTokens: u.CachedTokens}
	}
	return &OAIResponse{
		ID: id, Object: "chat.completion", Created: created, Model: model,
		Choices: []OAIChoice{{Index: 0, Message: msg, FinishReason: &finish}},
		Usage:   usage,
	}
}
