package translate

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAnthropicToOpenAI(t *testing.T) {
	body := `{
		"model": "claude-x",
		"max_tokens": 1024,
		"system": [{"type":"text","text":"You are terse."},{"type":"text","text":"Be kind."}],
		"stop_sequences": ["END"],
		"temperature": 0.3,
		"messages": [
			{"role":"user","content":"What's the weather?"},
			{"role":"assistant","content":[
				{"type":"thinking","thinking":"hmm","signature":"sig"},
				{"type":"text","text":"Let me check."},
				{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"city":"Dhaka"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"31C"}]},
				{"type":"text","text":"And tomorrow?"},
				{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAAA"}}
			]}
		],
		"tools": [{"name":"get_weather","description":"Weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}}}}],
		"tool_choice": {"type":"any","disable_parallel_tool_use":true}
	}`
	var req AnthRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	out, err := AnthropicToOpenAI(&req)
	if err != nil {
		t.Fatal(err)
	}
	if *out.MaxTokens != 1024 || string(out.Stop) != `["END"]` || *out.Temperature != 0.3 {
		t.Fatalf("scalar fields wrong: %+v", out)
	}
	if len(out.Messages) != 5 {
		t.Fatalf("want 5 messages (system,user,assistant,tool,user) got %d: %s", len(out.Messages), mustJSON(t, out.Messages))
	}
	if out.Messages[0].Role != "system" || !strings.Contains(string(out.Messages[0].Content), "You are terse.\\n\\nBe kind.") {
		t.Fatalf("system: %s", mustJSON(t, out.Messages[0]))
	}
	a := out.Messages[2]
	if a.Role != "assistant" || string(a.Content) != `"Let me check."` || len(a.ToolCalls) != 1 ||
		a.ToolCalls[0].Function.Arguments != `{"city":"Dhaka"}` || a.ToolCalls[0].ID != "toolu_1" {
		t.Fatalf("assistant: %s", mustJSON(t, a))
	}
	tool := out.Messages[3]
	if tool.Role != "tool" || tool.ToolCallID != "toolu_1" || string(tool.Content) != `"31C"` {
		t.Fatalf("tool: %s", mustJSON(t, tool))
	}
	u := out.Messages[4]
	if u.Role != "user" || !strings.Contains(string(u.Content), "data:image/png;base64,AAAA") {
		t.Fatalf("user with image: %s", mustJSON(t, u))
	}
	if len(out.Tools) != 1 || out.Tools[0].Function.Name != "get_weather" {
		t.Fatalf("tools: %s", mustJSON(t, out.Tools))
	}
	if string(out.ToolChoice) != `"required"` || out.ParallelToolCalls == nil || *out.ParallelToolCalls {
		t.Fatalf("tool choice: %s %v", out.ToolChoice, out.ParallelToolCalls)
	}
}

func TestOpenAIToAnthropic(t *testing.T) {
	body := `{
		"model":"gpt-x",
		"temperature": 1.6,
		"stop": "STOP",
		"messages":[
			{"role":"system","content":"sys1"},
			{"role":"developer","content":"sys2"},
			{"role":"user","content":[{"type":"text","text":"hi"},{"type":"image_url","image_url":{"url":"data:image/jpeg;base64,QUJD"}}]},
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_1","type":"function","function":{"name":"a","arguments":"{\"x\":1}"}},
				{"id":"call_2","type":"function","function":{"name":"b","arguments":"not json"}}
			]},
			{"role":"tool","tool_call_id":"call_1","content":"r1"},
			{"role":"tool","tool_call_id":"call_2","content":"r2"},
			{"role":"user","content":"thanks"}
		],
		"tools":[{"type":"function","function":{"name":"a","parameters":{"type":"object"}}}],
		"tool_choice":{"type":"function","function":{"name":"a"}}
	}`
	var req OAIRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	out, err := OpenAIToAnthropic(&req, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if out.MaxTokens != 4096 || *out.Temperature != 0.8 || out.StopSequences[0] != "STOP" {
		t.Fatalf("scalars: %+v", out)
	}
	if string(out.System) != `"sys1\n\nsys2"` {
		t.Fatalf("system: %s", out.System)
	}
	// user, assistant, user(tool results merged + "thanks")
	if len(out.Messages) != 3 {
		t.Fatalf("want 3 alternating messages, got %d: %s", len(out.Messages), mustJSON(t, out.Messages))
	}
	var blocks []AnthBlock
	_ = json.Unmarshal(out.Messages[0].Content, &blocks)
	if len(blocks) != 2 || blocks[1].Type != "image" || blocks[1].Source.MediaType != "image/jpeg" || blocks[1].Source.Data != "QUJD" {
		t.Fatalf("user blocks: %s", out.Messages[0].Content)
	}
	_ = json.Unmarshal(out.Messages[1].Content, &blocks)
	if len(blocks) != 2 || blocks[0].Type != "tool_use" || string(blocks[0].Input) != `{"x":1}` || string(blocks[1].Input) != `{}` {
		t.Fatalf("assistant blocks: %s", out.Messages[1].Content)
	}
	_ = json.Unmarshal(out.Messages[2].Content, &blocks)
	if len(blocks) != 3 || blocks[0].Type != "tool_result" || blocks[1].ToolUseID != "call_2" || blocks[2].Text != "thanks" {
		t.Fatalf("merged user blocks: %s", out.Messages[2].Content)
	}
	if out.ToolChoice == nil || out.ToolChoice.Type != "tool" || out.ToolChoice.Name != "a" {
		t.Fatalf("tool choice: %+v", out.ToolChoice)
	}
}

func TestResponseConversions(t *testing.T) {
	finish := "tool_calls"
	oai := &OAIResponse{
		Choices: []OAIChoice{{Message: &OAIMessage{Role: "assistant", Content: rawString("ok"),
			ToolCalls: []OAIToolCall{{ID: "c1", Function: OAIFunctionCall{Name: "f", Arguments: `{"a":2}`}}}}, FinishReason: &finish}},
		Usage: &OAIUsage{PromptTokens: 10, CompletionTokens: 5, PromptTokensDetails: &PromptTokenDetails{CachedTokens: 4}},
	}
	a := OpenAIResponseToAnthropic(oai, "pub/model", "msg_1")
	if a.Model != "pub/model" || *a.StopReason != "tool_use" || len(a.Content) != 2 || a.Content[1].Name != "f" ||
		a.Usage.InputTokens != 10 || a.Usage.OutputTokens != 5 || a.Usage.CacheReadInputTokens != 4 {
		t.Fatalf("oai->anth: %s", mustJSON(t, a))
	}

	stop := "max_tokens"
	anth := &AnthResponse{Content: []AnthBlock{{Type: "text", Text: "hello"}}, StopReason: &stop,
		Usage: AnthUsage{InputTokens: 3, OutputTokens: 7, CacheReadInputTokens: 2}}
	o := AnthropicResponseToOpenAI(anth, "pub/model", "chatcmpl-1", 1)
	if *o.Choices[0].FinishReason != "length" || string(o.Choices[0].Message.Content) != `"hello"` ||
		o.Usage.PromptTokens != 5 || o.Usage.CompletionTokens != 7 || o.Model != "pub/model" {
		t.Fatalf("anth->oai: %s", mustJSON(t, o))
	}
}

func TestInvalidContent(t *testing.T) {
	req := &AnthRequest{Messages: []AnthMessage{{Role: "user", Content: json.RawMessage(`123`)}}}
	if _, err := AnthropicToOpenAI(req); err == nil {
		t.Fatal("expected error for numeric content")
	}
	req2 := &AnthRequest{Messages: []AnthMessage{{Role: "system", Content: rawString("x")}}}
	if _, err := AnthropicToOpenAI(req2); err != nil {
		// string content short-circuits role validation; that's fine for upstream.
		t.Log(err)
	}
}
