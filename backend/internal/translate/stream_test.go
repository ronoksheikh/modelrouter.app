package translate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func readAll(t *testing.T, s string) []SSEEvent {
	t.Helper()
	r := NewSSEReader(strings.NewReader(s))
	var out []SSEEvent
	for {
		ev, err := r.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, ev)
	}
}

func TestSSEReader(t *testing.T) {
	evs := readAll(t, ": OPENROUTER PROCESSING\r\n\r\nevent: a\r\ndata: {\"x\":1}\r\n\r\ndata: line1\ndata: line2\n\ndata: [DONE]")
	if len(evs) != 4 {
		t.Fatalf("got %d events: %+v", len(evs), evs)
	}
	if !evs[0].Comment || evs[1].Event != "a" || string(evs[1].Data) != `{"x":1}` ||
		string(evs[2].Data) != "line1\nline2" || string(evs[3].Data) != "[DONE]" {
		t.Fatalf("bad parse: %+v", evs)
	}
}

func TestOpenAIStreamToAnthropic(t *testing.T) {
	chunks := []string{
		`{"choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}`,
		`{"choices":[{"index":0,"delta":{"content":"Hel"}}]}`,
		`{"choices":[{"index":0,"delta":{"content":"lo"}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"f","arguments":""}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"a\":"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`{"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`,
	}
	var buf bytes.Buffer
	c := NewOAIToAnthStream(&buf, "pub/m", "msg_1")
	for _, ch := range chunks {
		if err := c.Chunk([]byte(ch)); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Finish(); err != nil {
		t.Fatal(err)
	}
	evs := readAll(t, buf.String())
	var types []string
	for _, e := range evs {
		types = append(types, e.Event)
	}
	want := "message_start ping content_block_start content_block_delta content_block_delta content_block_stop " +
		"content_block_start content_block_delta content_block_delta content_block_stop message_delta message_stop"
	if strings.Join(types, " ") != want {
		t.Fatalf("event sequence:\n got %s\nwant %s", strings.Join(types, " "), want)
	}
	var md struct {
		Delta struct {
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
		Usage AnthUsage `json:"usage"`
	}
	_ = json.Unmarshal(evs[10].Data, &md)
	if md.Delta.StopReason != "tool_use" || md.Usage.OutputTokens != 8 || md.Usage.InputTokens != 12 {
		t.Fatalf("message_delta: %s", evs[10].Data)
	}
	if !strings.Contains(string(evs[7].Data), `"partial_json":"{\"a\":"`) {
		t.Fatalf("tool delta: %s", evs[7].Data)
	}
	if u := c.Usage(); u.InputTokens != 12 || u.OutputTokens != 8 {
		t.Fatalf("usage %+v", u)
	}
}

func TestOpenAIStreamEmptyStillValid(t *testing.T) {
	var buf bytes.Buffer
	c := NewOAIToAnthStream(&buf, "m", "msg")
	_ = c.Finish()
	evs := readAll(t, buf.String())
	if len(evs) != 6 || evs[0].Event != "message_start" || evs[5].Event != "message_stop" {
		t.Fatalf("got %+v", evs)
	}
}

func TestAnthropicStreamToOpenAI(t *testing.T) {
	events := []struct{ ev, data string }{
		{"message_start", `{"type":"message_start","message":{"id":"x","usage":{"input_tokens":20,"output_tokens":1}}}`},
		{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
		{"ping", `{"type":"ping"}`},
		{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`},
		{"content_block_stop", `{"type":"content_block_stop","index":0}`},
		{"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_9","name":"f","input":{}}}`},
		{"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"q\":1}"}}`},
		{"content_block_stop", `{"type":"content_block_stop","index":1}`},
		{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":15}}`},
		{"message_stop", `{"type":"message_stop"}`},
	}
	var buf bytes.Buffer
	c := NewAnthToOAIStream(&buf, "pub/m", "chatcmpl-1", 1, true)
	for _, e := range events {
		if err := c.Event(e.ev, []byte(e.data)); err != nil {
			t.Fatal(err)
		}
	}
	_ = c.Finish() // idempotent
	evs := readAll(t, buf.String())
	last := evs[len(evs)-1]
	if string(last.Data) != "[DONE]" {
		t.Fatalf("missing [DONE]: %s", buf.String())
	}
	out := buf.String()
	for _, want := range []string{`"role":"assistant"`, `"content":"Hi"`, `"id":"toolu_9"`, `"arguments":"{\"q\":1}"`,
		`"finish_reason":"tool_calls"`, `"prompt_tokens":20`, `"completion_tokens":15`} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %s\n%s", want, out)
		}
	}
	if strings.Count(out, "[DONE]") != 1 {
		t.Fatal("[DONE] must be sent exactly once")
	}
}
