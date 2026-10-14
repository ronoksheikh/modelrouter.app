package translate

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
)

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// SSE reader
// ---------------------------------------------------------------------------

type SSEEvent struct {
	Event   string
	Data    []byte
	Comment bool
}

// SSEReader parses a text/event-stream. It tolerates CRLF line endings and
// very long data lines.
type SSEReader struct {
	r *bufio.Reader
}

func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{r: bufio.NewReaderSize(r, 64<<10)}
}

func (s *SSEReader) Next() (SSEEvent, error) {
	var ev SSEEvent
	var data bytes.Buffer
	hasData := false
	for {
		line, err := s.r.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			if hasData || ev.Event != "" {
				ev.Data = data.Bytes()
				return ev, nil
			}
			return ev, err
		}
		line = bytes.TrimRight(line, "\r\n")
		if len(line) == 0 {
			if hasData || ev.Event != "" {
				ev.Data = data.Bytes()
				return ev, nil
			}
			if err != nil {
				return ev, err
			}
			continue
		}
		switch {
		case line[0] == ':':
			if !hasData && ev.Event == "" {
				return SSEEvent{Comment: true, Data: append([]byte(nil), line[1:]...)}, nil
			}
		case bytes.HasPrefix(line, []byte("data:")):
			v := line[5:]
			if len(v) > 0 && v[0] == ' ' {
				v = v[1:]
			}
			if hasData {
				data.WriteByte('\n')
			}
			data.Write(v)
			hasData = true
		case bytes.HasPrefix(line, []byte("event:")):
			ev.Event = string(bytes.TrimSpace(line[6:]))
		}
		if err != nil {
			ev.Data = data.Bytes()
			return ev, nil
		}
	}
}

// WriteSSE writes a single event. An empty event name writes only data.
func WriteSSE(w io.Writer, event string, data []byte) error {
	var buf bytes.Buffer
	buf.Grow(len(data) + len(event) + 16)
	if event != "" {
		buf.WriteString("event: ")
		buf.WriteString(event)
		buf.WriteByte('\n')
	}
	buf.WriteString("data: ")
	buf.Write(data)
	buf.WriteString("\n\n")
	_, err := w.Write(buf.Bytes())
	return err
}

func writeJSONEvent(w io.Writer, event string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return WriteSSE(w, event, b)
}

// ---------------------------------------------------------------------------
// OpenAI chunks -> Anthropic events
// ---------------------------------------------------------------------------

type OAIToAnthStream struct {
	w     io.Writer
	model string
	id    string

	started    bool
	finished   bool
	blockOpen  bool
	blockType  string
	nextIndex  int
	toolBlocks map[int]int // OpenAI tool index -> Anthropic block index
	curTool    int
	stop       string
	usage      Usage
	gotContent bool
}

func NewOAIToAnthStream(w io.Writer, model, id string) *OAIToAnthStream {
	return &OAIToAnthStream{w: w, model: model, id: id, toolBlocks: map[int]int{}, curTool: -1}
}

func (c *OAIToAnthStream) Usage() Usage { return c.usage }

func (c *OAIToAnthStream) start() error {
	if c.started {
		return nil
	}
	c.started = true
	msg := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": c.id, "type": "message", "role": "assistant", "model": c.model, "content": []any{},
			"stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]int{"input_tokens": c.usage.InputTokens, "output_tokens": 0},
		},
	}
	if err := writeJSONEvent(c.w, "message_start", msg); err != nil {
		return err
	}
	return writeJSONEvent(c.w, "ping", map[string]string{"type": "ping"})
}

func (c *OAIToAnthStream) closeBlock() error {
	if !c.blockOpen {
		return nil
	}
	c.blockOpen = false
	c.curTool = -1
	return writeJSONEvent(c.w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": c.nextIndex - 1})
}

func (c *OAIToAnthStream) openBlock(typ string, block map[string]any) error {
	if err := c.closeBlock(); err != nil {
		return err
	}
	c.blockOpen = true
	c.blockType = typ
	idx := c.nextIndex
	c.nextIndex++
	return writeJSONEvent(c.w, "content_block_start", map[string]any{"type": "content_block_start", "index": idx, "content_block": block})
}

type oaiChunk struct {
	Choices []OAIChoice     `json:"choices"`
	Usage   *OAIUsage       `json:"usage"`
	Error   json.RawMessage `json:"error"`
}

// Chunk processes one OpenAI `data:` payload (excluding "[DONE]").
func (c *OAIToAnthStream) Chunk(data []byte) error {
	var ch oaiChunk
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil // ignore malformed keep-alive payloads
	}
	if !isNull(ch.Error) {
		return c.Error(errorMessage(ch.Error))
	}
	if ch.Usage != nil {
		c.usage = UsageFromOpenAI(ch.Usage)
	}
	if err := c.start(); err != nil {
		return err
	}
	for _, choice := range ch.Choices {
		if choice.Index != 0 || choice.Delta == nil {
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				c.stop = MapFinishToStop(*choice.FinishReason)
			}
			continue
		}
		d := choice.Delta
		if d.Content != nil && *d.Content != "" {
			if !c.blockOpen || c.blockType != "text" {
				if err := c.openBlock("text", map[string]any{"type": "text", "text": ""}); err != nil {
					return err
				}
			}
			c.gotContent = true
			if err := writeJSONEvent(c.w, "content_block_delta", map[string]any{
				"type": "content_block_delta", "index": c.nextIndex - 1,
				"delta": map[string]string{"type": "text_delta", "text": *d.Content},
			}); err != nil {
				return err
			}
		}
		for _, tc := range d.ToolCalls {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}
			if _, seen := c.toolBlocks[idx]; !seen {
				if err := c.openBlock("tool_use", map[string]any{
					"type": "tool_use", "id": toolID(tc.ID), "name": tc.Function.Name, "input": map[string]any{},
				}); err != nil {
					return err
				}
				c.toolBlocks[idx] = c.nextIndex - 1
				c.curTool = idx
				c.gotContent = true
			}
			if tc.Function.Arguments != "" && c.curTool == idx && c.blockOpen {
				if err := writeJSONEvent(c.w, "content_block_delta", map[string]any{
					"type": "content_block_delta", "index": c.toolBlocks[idx],
					"delta": map[string]string{"type": "input_json_delta", "partial_json": tc.Function.Arguments},
				}); err != nil {
					return err
				}
			}
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			c.stop = MapFinishToStop(*choice.FinishReason)
		}
	}
	return nil
}

// Error emits an Anthropic error event and ends the stream.
func (c *OAIToAnthStream) Error(msg string) error {
	if c.finished {
		return nil
	}
	c.finished = true
	return writeJSONEvent(c.w, "error", map[string]any{"type": "error", "error": map[string]string{"type": "api_error", "message": msg}})
}

// Finish closes open blocks and emits message_delta + message_stop.
func (c *OAIToAnthStream) Finish() error {
	if c.finished {
		return nil
	}
	if err := c.start(); err != nil {
		return err
	}
	if !c.gotContent {
		// Always emit at least one (empty) text block, as Anthropic does.
		if err := c.openBlock("text", map[string]any{"type": "text", "text": ""}); err != nil {
			return err
		}
	}
	if err := c.closeBlock(); err != nil {
		return err
	}
	c.finished = true
	stop := c.stop
	if stop == "" {
		stop = "end_turn"
	}
	if err := writeJSONEvent(c.w, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stop, "stop_sequence": nil},
		"usage": map[string]int{"input_tokens": c.usage.InputTokens, "output_tokens": c.usage.OutputTokens},
	}); err != nil {
		return err
	}
	return writeJSONEvent(c.w, "message_stop", map[string]string{"type": "message_stop"})
}

func errorMessage(raw json.RawMessage) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Message != "" {
		return e.Message
	}
	if s, ok := contentString(raw); ok {
		return s
	}
	return "upstream error"
}

// ---------------------------------------------------------------------------
// Anthropic events -> OpenAI chunks
// ---------------------------------------------------------------------------

type AnthToOAIStream struct {
	w            io.Writer
	id           string
	model        string
	created      int64
	includeUsage bool

	sentRole  bool
	finished  bool
	toolIndex map[int]int // Anthropic block index -> OpenAI tool index
	nextTool  int
	finish    string
	usage     Usage
}

func NewAnthToOAIStream(w io.Writer, model, id string, created int64, includeUsage bool) *AnthToOAIStream {
	return &AnthToOAIStream{w: w, model: model, id: id, created: created, includeUsage: includeUsage, toolIndex: map[int]int{}}
}

func (c *AnthToOAIStream) Usage() Usage { return c.usage }

func (c *AnthToOAIStream) chunk(delta map[string]any, finish *string, usage *OAIUsage) error {
	choices := []any{}
	if delta != nil || finish != nil {
		if delta == nil {
			delta = map[string]any{}
		}
		choices = append(choices, map[string]any{"index": 0, "delta": delta, "finish_reason": finish})
	}
	m := map[string]any{"id": c.id, "object": "chat.completion.chunk", "created": c.created, "model": c.model, "choices": choices}
	if usage != nil {
		m["usage"] = usage
	}
	return writeJSONEvent(c.w, "", m)
}

func (c *AnthToOAIStream) ensureRole() error {
	if c.sentRole {
		return nil
	}
	c.sentRole = true
	return c.chunk(map[string]any{"role": "assistant", "content": ""}, nil, nil)
}

type anthEvent struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message *struct {
		Usage AnthUsage `json:"usage"`
	} `json:"message"`
	ContentBlock *AnthBlock `json:"content_block"`
	Delta        *struct {
		Type        string  `json:"type"`
		Text        string  `json:"text"`
		PartialJSON string  `json:"partial_json"`
		Thinking    string  `json:"thinking"`
		StopReason  *string `json:"stop_reason"`
	} `json:"delta"`
	Usage *AnthUsage      `json:"usage"`
	Error json.RawMessage `json:"error"`
}

func (c *AnthToOAIStream) Event(_ string, data []byte) error {
	var ev anthEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil
	}
	switch ev.Type {
	case "message_start":
		if ev.Message != nil {
			c.usage = UsageFromAnthropic(ev.Message.Usage)
		}
		return c.ensureRole()
	case "content_block_start":
		if err := c.ensureRole(); err != nil {
			return err
		}
		if ev.ContentBlock != nil && ev.ContentBlock.Type == "tool_use" {
			ti := c.nextTool
			c.nextTool++
			c.toolIndex[ev.Index] = ti
			return c.chunk(map[string]any{"tool_calls": []any{map[string]any{
				"index": ti, "id": ev.ContentBlock.ID, "type": "function",
				"function": map[string]string{"name": ev.ContentBlock.Name, "arguments": ""},
			}}}, nil, nil)
		}
	case "content_block_delta":
		if ev.Delta == nil {
			return nil
		}
		if err := c.ensureRole(); err != nil {
			return err
		}
		switch ev.Delta.Type {
		case "text_delta":
			return c.chunk(map[string]any{"content": ev.Delta.Text}, nil, nil)
		case "thinking_delta":
			return c.chunk(map[string]any{"reasoning": ev.Delta.Thinking}, nil, nil)
		case "input_json_delta":
			ti, ok := c.toolIndex[ev.Index]
			if !ok {
				return nil
			}
			return c.chunk(map[string]any{"tool_calls": []any{map[string]any{
				"index": ti, "function": map[string]string{"arguments": ev.Delta.PartialJSON},
			}}}, nil, nil)
		}
	case "message_delta":
		if ev.Delta != nil && ev.Delta.StopReason != nil {
			c.finish = MapStopToFinish(*ev.Delta.StopReason)
		}
		if ev.Usage != nil {
			c.usage.OutputTokens = ev.Usage.OutputTokens
			if ev.Usage.InputTokens > 0 {
				c.usage = Usage{
					InputTokens:  ev.Usage.InputTokens + ev.Usage.CacheReadInputTokens + ev.Usage.CacheCreationInputTokens,
					OutputTokens: ev.Usage.OutputTokens, CachedTokens: ev.Usage.CacheReadInputTokens,
				}
			}
		}
	case "message_stop":
		return c.Finish()
	case "error":
		return c.Error(errorMessage(ev.Error))
	}
	return nil
}

func (c *AnthToOAIStream) Error(msg string) error {
	if c.finished {
		return nil
	}
	c.finished = true
	if err := writeJSONEvent(c.w, "", map[string]any{"error": map[string]any{"message": msg, "type": "server_error"}}); err != nil {
		return err
	}
	return WriteSSE(c.w, "", []byte("[DONE]"))
}

func (c *AnthToOAIStream) Finish() error {
	if c.finished {
		return nil
	}
	c.finished = true
	if err := c.ensureRole(); err != nil {
		return err
	}
	finish := c.finish
	if finish == "" {
		finish = "stop"
	}
	if err := c.chunk(map[string]any{}, &finish, nil); err != nil {
		return err
	}
	if c.includeUsage {
		u := &OAIUsage{PromptTokens: c.usage.InputTokens, CompletionTokens: c.usage.OutputTokens,
			TotalTokens: c.usage.InputTokens + c.usage.OutputTokens}
		if c.usage.CachedTokens > 0 {
			u.PromptTokensDetails = &PromptTokenDetails{CachedTokens: c.usage.CachedTokens}
		}
		if err := c.chunk(nil, nil, u); err != nil {
			return err
		}
	}
	return WriteSSE(c.w, "", []byte("[DONE]"))
}
