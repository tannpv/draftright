// Package anthropic implements domain.AiProvider against the
// Anthropic Messages API in streaming mode.
//
// Wire format (Anthropic SSE):
//
//	event: message_start
//	data: {"type":"message_start","message":{…}}
//
//	event: content_block_delta
//	data: {"type":"content_block_delta","index":0,
//	       "delta":{"type":"text_delta","text":"Hello"}}
//
//	event: message_stop
//	data: {"type":"message_stop"}
//
// We forward `delta.text` for `content_block_delta` events and ignore
// everything else. Same shape as the OpenAI adapter so the wire-level
// behaviour stays consistent between providers (Rule #1 — same
// failure modes, same goroutine lifecycle).
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
)

const (
	defaultEndpoint       = "https://api.anthropic.com/v1/messages"
	defaultModel          = "claude-3-5-sonnet-20241022"
	defaultMaxTokens      = 1024
	defaultAPIVersion     = "2023-06-01"
	defaultRequestTimeout = 120 * time.Second
)

// Client is the Anthropic provider. Safe for concurrent use.
type Client struct {
	id         uuid.UUID
	apiKey     string
	endpoint   string
	model      string
	maxTokens  int
	apiVersion string
	http       *http.Client
	systemFmt  func(domain.Tone) string
}

// Option configures the Client; functional-options pattern matches
// the OpenAI adapter so swap-in tests are uniform.
type Option func(*Client)

// WithEndpoint overrides the default API URL (useful for the Anthropic
// proxy / test httptest server).
func WithEndpoint(url string) Option { return func(c *Client) { c.endpoint = url } }

// WithModel pins a specific model.
func WithModel(m string) Option { return func(c *Client) { c.model = m } }

// WithMaxTokens caps generation length. Anthropic requires the field.
func WithMaxTokens(n int) Option { return func(c *Client) { c.maxTokens = n } }

// WithHTTPClient injects a custom *http.Client (tests use httptest).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithSystemPromptFn lets the caller pick the system prompt per Tone.
func WithSystemPromptFn(fn func(domain.Tone) string) Option {
	return func(c *Client) { c.systemFmt = fn }
}

// New builds a Client. id is recorded on every usage_logs row.
func New(id uuid.UUID, apiKey string, opts ...Option) *Client {
	c := &Client{
		id:         id,
		apiKey:     apiKey,
		endpoint:   defaultEndpoint,
		model:      defaultModel,
		maxTokens:  defaultMaxTokens,
		apiVersion: defaultAPIVersion,
		http:       &http.Client{Timeout: defaultRequestTimeout},
		systemFmt:  DefaultSystemPrompt,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ID returns the ai_providers.id UUID.
func (c *Client) ID() uuid.UUID { return c.id }

// Name identifies the provider in logs.
func (c *Client) Name() string { return "anthropic" }

// Stream POSTs to /v1/messages with stream=true, parses the SSE
// response, forwards text_delta tokens.
func (c *Client) Stream(ctx context.Context, req domain.RewriteRequest) (<-chan string, <-chan error) {
	tokens := make(chan string)
	errs := make(chan error, 1)

	body, marshalErr := json.Marshal(c.buildRequest(req))
	if marshalErr != nil {
		errs <- fmt.Errorf("anthropic: marshal request: %w", marshalErr)
		close(tokens)
		close(errs)
		return tokens, errs
	}

	go func() {
		defer close(tokens)
		defer close(errs)

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			errs <- fmt.Errorf("anthropic: build request: %w", err)
			return
		}
		httpReq.Header.Set("x-api-key", c.apiKey)
		httpReq.Header.Set("anthropic-version", c.apiVersion)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := c.http.Do(httpReq)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				errs <- err
				return
			}
			errs <- fmt.Errorf("anthropic: do request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode/100 != 2 {
			errs <- fmt.Errorf("anthropic: %s: %s", resp.Status, readBriefly(resp.Body))
			return
		}

		c.parseSSE(ctx, resp.Body, tokens, errs)
	}()

	return tokens, errs
}

// parseSSE walks the SSE stream. Anthropic emits an `event:` line
// before each `data:` line. We track the most recent event name; only
// content_block_delta payloads produce tokens.
func (c *Client) parseSSE(ctx context.Context, body io.Reader, tokens chan<- string, errs chan<- error) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)

	var currentEvent string
	for sc.Scan() {
		if ctx.Err() != nil {
			errs <- ctx.Err()
			return
		}
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
			// Event separator; reset the running event name.
			currentEvent = ""
		case strings.HasPrefix(line, "event:"):
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			// Anthropic's only terminal-ish marker is message_stop.
			// We could also see [DONE] from proxies; treat both as
			// end-of-stream.
			if payload == "[DONE]" {
				return
			}
			if currentEvent != "content_block_delta" {
				continue
			}
			var chunk deltaChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}
			if chunk.Delta.Type != "text_delta" || chunk.Delta.Text == "" {
				continue
			}
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			case tokens <- chunk.Delta.Text:
			}
		}
	}
	if err := sc.Err(); err != nil {
		errs <- fmt.Errorf("anthropic: read stream: %w", err)
	}
}

// buildRequest assembles the messages payload. Anthropic puts the
// system prompt at the top level, not inside the messages array.
func (c *Client) buildRequest(req domain.RewriteRequest) map[string]any {
	return map[string]any{
		"model":      c.model,
		"max_tokens": c.maxTokens,
		"stream":     true,
		"system":     c.systemFmt(req.Tone()),
		"messages": []map[string]string{
			{"role": "user", "content": req.Text()},
		},
	}
}

func readBriefly(r io.Reader) string {
	var b strings.Builder
	const max = 2048
	_, _ = io.Copy(&b, io.LimitReader(r, max))
	return b.String()
}

// deltaChunk is the subset of Anthropic's content_block_delta payload
// we care about.
type deltaChunk struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

// DefaultSystemPrompt mirrors openai's DefaultSystemPrompt so every
// provider applies the same per-tone instruction. Diverges only when
// a provider's prompt-engineering needs warrant it (Anthropic prefers
// XML wrappers — TBD if rewrite quality demands).
func DefaultSystemPrompt(tone domain.Tone) string {
	switch tone {
	case domain.ToneSimple:
		return "Rewrite the text in simple, clear language."
	case domain.ToneNatural:
		return "Rewrite the text to sound natural and conversational."
	case domain.TonePolished:
		return "Rewrite the text to be polished and professional."
	case domain.ToneConcise:
		return "Rewrite the text to be more concise without losing meaning."
	case domain.ToneTechnical:
		return "Rewrite the text using precise technical language."
	case domain.ToneClaude:
		return "Rewrite the text in a thoughtful, considered tone."
	case domain.ToneTranslate:
		return "Translate the text accurately, preserving tone."
	default:
		return "Rewrite the text."
	}
}
