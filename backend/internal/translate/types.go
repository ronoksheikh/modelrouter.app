// Package translate converts between the OpenAI Chat Completions and the
// Anthropic Messages wire formats, including streaming (SSE) events.
package translate

import "encoding/json"

// ---------- OpenAI ----------

type OAIRequest struct {
	Model               string          `json:"model"`
	Messages            []OAIMessage    `json:"messages"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	TopK                *int            `json:"top_k,omitempty"`
	Stop                json.RawMessage `json:"stop,omitempty"`
	Stream              bool            `json:"stream,omitempty"`
	StreamOptions       *StreamOptions  `json:"stream_options,omitempty"`
	Tools               []OAITool       `json:"tools,omitempty"`
	ToolChoice          json.RawMessage `json:"tool_choice,omitempty"`
	ParallelToolCalls   *bool           `json:"parallel_tool_calls,omitempty"`
	User                string          `json:"user,omitempty"`
	Reasoning           json.RawMessage `json:"reasoning,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type OAIMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content,omitempty"` // string | []OAIPart | null
	Name       string          `json:"name,omitempty"`
	ToolCalls  []OAIToolCall   `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Reasoning  string          `json:"reasoning,omitempty"`
}

type OAIPart struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *OAIImageURL `json:"image_url,omitempty"`
}

type OAIImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type OAITool struct {
	Type     string      `json:"type"`
	Function OAIFunction `json:"function"`
}

type OAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type OAIToolCall struct {
	Index    *int            `json:"index,omitempty"`
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type,omitempty"`
	Function OAIFunctionCall `json:"function"`
}

type OAIFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

type OAIResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
	Model   string      `json:"model"`
	Choices []OAIChoice `json:"choices"`
	Usage   *OAIUsage   `json:"usage,omitempty"`
}

type OAIChoice struct {
	Index        int         `json:"index"`
	Message      *OAIMessage `json:"message,omitempty"`
	Delta        *OAIDelta   `json:"delta,omitempty"`
	FinishReason *string     `json:"finish_reason"`
}

type OAIDelta struct {
	Role      string        `json:"role,omitempty"`
	Content   *string       `json:"content,omitempty"`
	Reasoning *string       `json:"reasoning,omitempty"`
	ToolCalls []OAIToolCall `json:"tool_calls,omitempty"`
}

type OAIUsage struct {
	PromptTokens            int                 `json:"prompt_tokens"`
	CompletionTokens        int                 `json:"completion_tokens"`
	TotalTokens             int                 `json:"total_tokens"`
	PromptTokensDetails     *PromptTokenDetails `json:"prompt_tokens_details,omitempty"`
	Cost                    *float64            `json:"cost,omitempty"`
	CompletionTokensDetails json.RawMessage     `json:"completion_tokens_details,omitempty"`
}

type PromptTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// ---------- Anthropic ----------

type AnthRequest struct {
	Model         string          `json:"model"`
	Messages      []AnthMessage   `json:"messages"`
	System        json.RawMessage `json:"system,omitempty"` // string | []AnthBlock
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Tools         []AnthTool      `json:"tools,omitempty"`
	ToolChoice    *AnthToolChoice `json:"tool_choice,omitempty"`
	Metadata      *struct {
		UserID string `json:"user_id,omitempty"`
	} `json:"metadata,omitempty"`
	Thinking *AnthThinking `json:"thinking,omitempty"`
}

type AnthThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

type AnthMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string | []AnthBlock
}

type AnthBlock struct {
	Type string `json:"type"`
	// text
	Text string `json:"text,omitempty"`
	// image / document
	Source *AnthSource `json:"source,omitempty"`
	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// tool_result
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	// thinking
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
}

type AnthSource struct {
	Type      string `json:"type"` // base64 | url | text
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type AnthTool struct {
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type AnthToolChoice struct {
	Type                   string `json:"type"` // auto | any | tool | none
	Name                   string `json:"name,omitempty"`
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
}

type AnthResponse struct {
	ID           string      `json:"id"`
	Type         string      `json:"type"`
	Role         string      `json:"role"`
	Model        string      `json:"model"`
	Content      []AnthBlock `json:"content"`
	StopReason   *string     `json:"stop_reason"`
	StopSequence *string     `json:"stop_sequence"`
	Usage        AnthUsage   `json:"usage"`
}

type AnthUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// Usage is the provider-neutral usage reported back to billing.
type Usage struct {
	InputTokens  int
	OutputTokens int
	CachedTokens int
	// UpstreamCostUSD is the upstream-reported cost (OpenRouter usage accounting), if any.
	UpstreamCostUSD *float64
}

func (u Usage) Known() bool { return u.InputTokens > 0 || u.OutputTokens > 0 }
