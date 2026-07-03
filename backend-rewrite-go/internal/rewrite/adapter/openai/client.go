// Package openai implements domain.AiProvider against the OpenAI
// chat-completions API in streaming mode. SSE chunks come back as
// `data: {…}\n\n` lines; this adapter parses each chunk's
// `choices[0].delta.content` and forwards the text token to the
// caller.
//
// ctx cancellation aborts the in-flight HTTP request — the upstream
// HTTP/2 stream is closed at the TCP layer so no further tokens
// arrive + no goroutine leaks.
//
// Why net/http (not the official openai-go SDK):
//   - The official SDK's streaming surface drags in a heavier set of
//     dependencies. Our needs are narrow: open a POST, parse SSE,
//     forward delta.content.
//   - Hand-rolling the wire format means we own the failure modes —
//     no surprise behaviour change on SDK bumps.
//   - When other providers (Anthropic, Ollama) land in Task 8 they
//     follow the same shape, so a shared "stream parser" emerges.
package openai

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
	"github.com/tannpv/draftright-rewrite/internal/rewrite/provenance"
)

const (
	// Default endpoint. Override via env when pointing at Azure
	// OpenAI / a proxy / a self-hosted shim.
	defaultEndpoint = "https://api.openai.com/v1/chat/completions"
	defaultModel    = "gpt-4o-mini"

	// Per-stream deadline. Long enough for slow networks + slow
	// models; short enough that a stalled upstream doesn't pin our
	// goroutine forever.
	defaultRequestTimeout = 120 * time.Second
)

// Client is the OpenAI provider. Construct once at startup; safe for
// concurrent use (it owns no per-request state).
type Client struct {
	id          uuid.UUID
	apiKey      string
	endpoint    string
	model       string
	temperature string // raw decimal(3,2) string, parsed at body-build (Node Number())
	isLocal     bool   // ollama type → isLocal message layout
	http        *http.Client
	systemFmt   func(domain.Tone) string
}

// Option configures the Client; canonical functional-options pattern
// so tests can supply a stub http.Client + the prod main can pin a
// specific model without exploding the constructor signature.
type Option func(*Client)

// WithEndpoint overrides the default OpenAI URL.
func WithEndpoint(url string) Option { return func(c *Client) { c.endpoint = url } }

// WithModel pins a specific model (default gpt-4o-mini).
func WithModel(m string) Option { return func(c *Client) { c.model = m } }

// WithTemperature stores the raw decimal(3,2) string (e.g. "0.30"), parsed at
// body-build time to mirror Node's Number(provider.temperature).
func WithTemperature(t string) Option { return func(c *Client) { c.temperature = t } }

// WithLocalLayout selects the ollama isLocal message layout (Node
// type===OLLAMA): a fixed assistant system message + the system prompt folded
// into the user message.
func WithLocalLayout(local bool) Option { return func(c *Client) { c.isLocal = local } }

// WithHTTPClient injects a custom *http.Client (tests use httptest).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithSystemPromptFn lets the caller pick the system prompt per Tone.
// Defaults to nestjs-parity (see DefaultSystemPrompt below).
func WithSystemPromptFn(fn func(domain.Tone) string) Option {
	return func(c *Client) { c.systemFmt = fn }
}

// New builds a Client. id is recorded on every usage_logs row so
// analytics can attribute load to a provider.
func New(id uuid.UUID, apiKey string, opts ...Option) *Client {
	c := &Client{
		id:        id,
		apiKey:    apiKey,
		endpoint:  defaultEndpoint,
		model:     defaultModel,
		http:      &http.Client{Timeout: defaultRequestTimeout},
		systemFmt: DefaultSystemPrompt,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ID returns the ai_providers.id UUID (for usage_logs.ai_provider_id).
func (c *Client) ID() uuid.UUID { return c.id }

// Name identifies the provider in slog + metrics.
func (c *Client) Name() string { return "openai" }

// Stream sends the chat-completions request with stream=true. Returns
// two channels — tokens + a terminal error — and a goroutine that
// closes both when the stream ends, errors, or is canceled.
func (c *Client) Stream(ctx context.Context, req domain.RewriteRequest) (<-chan string, <-chan error) {
	provenance.Stamp(ctx, c.model, c.Name())

	tokens := make(chan string)
	errs := make(chan error, 1)

	body, marshalErr := json.Marshal(c.buildRequest(req))
	if marshalErr != nil {
		// Producer goroutine never starts — close synchronously so
		// the use case's drain loop doesn't deadlock.
		errs <- fmt.Errorf("openai: marshal request: %w", marshalErr)
		close(tokens)
		close(errs)
		return tokens, errs
	}

	go func() {
		defer close(tokens)
		defer close(errs)

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			errs <- fmt.Errorf("openai: build request: %w", err)
			return
		}
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := c.http.Do(httpReq)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				errs <- err
				return
			}
			errs <- fmt.Errorf("openai: do request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode/100 != 2 {
			errs <- fmt.Errorf("openai: %s: %s", resp.Status, readBriefly(resp.Body))
			return
		}

		c.parseSSE(ctx, resp.Body, tokens, errs)
	}()

	return tokens, errs
}

// parseSSE walks the SSE stream line-by-line, decodes each data: chunk
// + forwards the delta.content. Stops on `data: [DONE]` (OpenAI's
// terminal marker) or when the body closes.
func (c *Client) parseSSE(ctx context.Context, body io.Reader, tokens chan<- string, errs chan<- error) {
	sc := bufio.NewScanner(body)
	// Bigger buffer than the default 64 KB — single chunks can be
	// large when the model emits long tool-call payloads.
	sc.Buffer(make([]byte, 0, 4096), 1<<20)

	for sc.Scan() {
		if ctx.Err() != nil {
			errs <- ctx.Err()
			return
		}
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue // ignore comments + blank separators
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			// Skip malformed chunks rather than fail the whole
			// stream — OpenAI occasionally inserts comment lines.
			continue
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content == "" {
				continue
			}
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			case tokens <- ch.Delta.Content:
			}
		}
	}
	if err := sc.Err(); err != nil {
		errs <- fmt.Errorf("openai: read stream: %w", err)
	}
}

// buildRequest assembles the chat-completions payload. Single seam
// for switching prompts per tone (system message) + adding new
// parameters like temperature later.
func (c *Client) buildRequest(req domain.RewriteRequest) map[string]any {
	return map[string]any{
		"model":  c.model,
		"stream": true,
		"messages": []map[string]string{
			{"role": "system", "content": domain.ApplySpeechPreamble(c.systemFmt(req.Tone()), req.InputKind())},
			{"role": "user", "content": req.Text()},
		},
	}
}

// readBriefly grabs at most 2 KB of an error body. Used for the
// "openai: 429: ..." error message so it's debuggable without
// pulling the entire upstream response into RAM.
func readBriefly(r io.Reader) string {
	var b strings.Builder
	const max = 2048
	_, _ = io.Copy(&b, io.LimitReader(r, max))
	return b.String()
}

// streamChunk is the relevant subset of OpenAI's SSE chunk shape.
// Unmarshalled per data: line.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// DefaultSystemPrompt is the fallback tone → system prompt mapping.
// Mirrors the prompts in NestJS rewrite.service.ts (tone enum) — keep
// in sync there + here. Future task: load prompts from the
// ai_providers row or a config file so they're not duplicated across
// services.
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
