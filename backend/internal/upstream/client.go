// Package upstream talks to model providers (OpenRouter, any OpenAI-compatible
// API, or Anthropic's native API).
package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/billing"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
)

const (
	KindOpenAI    = "openai"
	KindAnthropic = "anthropic"
)

// Provider is a resolved provider with its decrypted API key.
type Provider struct {
	ID           int64
	Name         string
	Kind         string
	BaseURL      string
	APIKey       string
	Headers      map[string]string
	IsOpenRouter bool
}

type Client struct {
	HTTP    *http.Client
	AppName string
	AppURL  string
}

func NewClient(timeout time.Duration, appName, appURL string) *Client {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1024,
		MaxIdleConnsPerHost:   512,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 5 * time.Minute, // reasoning models can take a while before the first byte
		ExpectContinueTimeout: time.Second,
		DisableCompression:    true, // stream bytes through untouched
	}
	return &Client{HTTP: &http.Client{Transport: tr, Timeout: timeout}, AppName: appName, AppURL: appURL}
}

// Endpoint returns the chat endpoint path for a provider kind.
func Endpoint(kind string) string {
	if kind == KindAnthropic {
		return "/messages"
	}
	return "/chat/completions"
}

func (c *Client) newRequest(ctx context.Context, p *Provider, method, path string, body []byte) (*http.Request, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.BaseURL+path, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch p.Kind {
	case KindAnthropic:
		req.Header.Set("x-api-key", p.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		if p.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.APIKey)
		}
	}
	if p.IsOpenRouter {
		req.Header.Set("HTTP-Referer", c.AppURL)
		req.Header.Set("X-Title", c.AppName)
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

// Chat sends a chat request. The caller owns the response body.
func (c *Client) Chat(ctx context.Context, p *Provider, body []byte, stream bool) (*http.Response, error) {
	req, err := c.newRequest(ctx, p, http.MethodPost, Endpoint(p.Kind), body)
	if err != nil {
		return nil, err
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	return c.HTTP.Do(req)
}

// Retryable reports whether an upstream status should be retried on a fallback.
func Retryable(status int) bool {
	return status == 408 || status == 429 || status == 502 || status == 503 || status == 504 || status == 529 || status >= 520
}

var brandRe = regexp.MustCompile(`(?i)open\s*router(\.ai)?`)

// Sanitize removes upstream vendor names from messages shown to customers.
func Sanitize(s string) string {
	s = brandRe.ReplaceAllString(s, "upstream")
	if len(s) > 1000 {
		s = s[:1000]
	}
	return s
}

// ErrorMessage extracts a human-readable message from an upstream error body.
func ErrorMessage(body []byte) string {
	var e struct {
		Error json.RawMessage `json:"error"`
		Msg   string          `json:"message"`
	}
	if json.Unmarshal(body, &e) == nil {
		var inner struct {
			Message string `json:"message"`
		}
		if len(e.Error) > 0 && json.Unmarshal(e.Error, &inner) == nil && inner.Message != "" {
			return Sanitize(inner.Message)
		}
		var s string
		if len(e.Error) > 0 && json.Unmarshal(e.Error, &s) == nil && s != "" {
			return Sanitize(s)
		}
		if e.Msg != "" {
			return Sanitize(e.Msg)
		}
	}
	if len(body) == 0 {
		return "upstream error"
	}
	return Sanitize(strings.TrimSpace(string(body)))
}

// ---------------------------------------------------------------------------
// Catalog sync
// ---------------------------------------------------------------------------

type orModel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Created       int64  `json:"created"`
	ContextLength int    `json:"context_length"`
	Pricing       struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
	TopProvider struct {
		ContextLength       *int `json:"context_length"`
		MaxCompletionTokens *int `json:"max_completion_tokens"`
	} `json:"top_provider"`
	Architecture struct {
		InputModalities []string `json:"input_modalities"`
	} `json:"architecture"`
	SupportedParameters []string `json:"supported_parameters"`
	// Anthropic /v1/models
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

// FetchModels downloads the provider's model catalog.
func (c *Client) FetchModels(ctx context.Context, p *Provider) ([]store.UpstreamModel, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	path := "/models"
	if p.Kind == KindAnthropic {
		path = "/models?limit=1000"
	}
	req, err := c.newRequest(ctx, p, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch models: status %d: %s", resp.StatusCode, ErrorMessage(body))
	}
	var payload struct {
		Data []orModel `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	if len(payload.Data) == 0 {
		return nil, errors.New("fetch models: empty catalog")
	}
	out := make([]store.UpstreamModel, 0, len(payload.Data))
	for _, m := range payload.Data {
		if m.ID == "" {
			continue
		}
		in, err1 := billing.PerTokenUSDToPerM(m.Pricing.Prompt)
		outp, err2 := billing.PerTokenUSDToPerM(m.Pricing.Completion)
		if err1 != nil || err2 != nil {
			// Variable-priced routers (e.g. "-1") cannot be billed safely; skip them.
			continue
		}
		um := store.UpstreamModel{
			ProviderID:    p.ID,
			UpstreamID:    m.ID,
			Name:          firstNonEmpty(m.Name, m.DisplayName, m.ID),
			Description:   m.Description,
			ContextLength: m.ContextLength,
			InputPrice:    in,
			OutputPrice:   outp,
			Modalities:    m.Architecture.InputModalities,
		}
		if m.TopProvider.MaxCompletionTokens != nil {
			um.MaxOutputTokens = *m.TopProvider.MaxCompletionTokens
		}
		if um.ContextLength == 0 && m.TopProvider.ContextLength != nil {
			um.ContextLength = *m.TopProvider.ContextLength
		}
		for _, sp := range m.SupportedParameters {
			if sp == "tools" {
				um.SupportsTools = true
			}
		}
		if p.Kind == KindAnthropic {
			um.SupportsTools = true
			um.Modalities = []string{"text", "image"}
		}
		if m.Created > 0 {
			t := time.Unix(m.Created, 0).UTC()
			um.UpstreamCreated = &t
		} else if m.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
				um.UpstreamCreated = &t
			}
		}
		if len(um.Modalities) == 0 {
			um.Modalities = []string{"text"}
		}
		out = append(out, um)
	}
	return out, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
