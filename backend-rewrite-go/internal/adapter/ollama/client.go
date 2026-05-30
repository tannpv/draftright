// Package ollama implements domain.AiProvider against a local Ollama
// server (default http://localhost:11434).
//
// Wire format: newline-delimited JSON. Each line is one chunk of the
// form:
//
//	{"model":"llama3.2","message":{"role":"assistant","content":"Hi"},"done":false}
//	{"model":"llama3.2","message":{"role":"assistant","content":" there"},"done":false}
//	{"model":"llama3.2","done":true,"total_duration":1234}
//
// No auth (local-only by default). If Ollama gets exposed behind a
// reverse proxy with bearer auth, swap in WithBearerToken later.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tannpv/draftright-rewrite/internal/domain"
)

const (
	defaultEndpoint       = "http://localhost:11434/api/chat"
	defaultModel          = "llama3.2"
	defaultRequestTimeout = 180 * time.Second // local models can be slow on CPU
)

// Client is the Ollama provider. Safe for concurrent use.
type Client struct {
	id        uuid.UUID
	endpoint  string
	model     string
	http      *http.Client
	systemFmt func(domain.Tone) string
}

// Option configures the Client.
type Option func(*Client)

// WithEndpoint overrides the default /api/chat URL.
func WithEndpoint(url string) Option { return func(c *Client) { c.endpoint = url } }

// WithModel pins a specific model (default llama3.2).
func WithModel(m string) Option { return func(c *Client) { c.model = m } }

// WithHTTPClient injects a custom *http.Client (tests use httptest).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithSystemPromptFn lets the caller pick the system prompt per Tone.
func WithSystemPromptFn(fn func(domain.Tone) string) Option {
	return func(c *Client) { c.systemFmt = fn }
}

// New builds a Client.
func New(id uuid.UUID, opts ...Option) *Client {
	c := &Client{
		id:        id,
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

// ID returns the ai_providers.id UUID.
func (c *Client) ID() uuid.UUID { return c.id }

// Name identifies the provider in logs.
func (c *Client) Name() string { return "ollama" }

// Stream POSTs to /api/chat with stream=true, parses NDJSON, forwards
// message.content tokens.
func (c *Client) Stream(ctx context.Context, req domain.RewriteRequest) (<-chan string, <-chan error) {
	tokens := make(chan string)
	errs := make(chan error, 1)

	body, marshalErr := json.Marshal(c.buildRequest(req))
	if marshalErr != nil {
		errs <- fmt.Errorf("ollama: marshal request: %w", marshalErr)
		close(tokens)
		close(errs)
		return tokens, errs
	}

	go func() {
		defer close(tokens)
		defer close(errs)

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			errs <- fmt.Errorf("ollama: build request: %w", err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(httpReq)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				errs <- err
				return
			}
			errs <- fmt.Errorf("ollama: do request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode/100 != 2 {
			errs <- fmt.Errorf("ollama: %s: %s", resp.Status, readBriefly(resp.Body))
			return
		}

		c.parseNDJSON(ctx, resp.Body, tokens, errs)
	}()

	return tokens, errs
}

// parseNDJSON reads one JSON object per line, forwards
// message.content, stops on done=true.
func (c *Client) parseNDJSON(ctx context.Context, body io.Reader, tokens chan<- string, errs chan<- error) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)

	for sc.Scan() {
		if ctx.Err() != nil {
			errs <- ctx.Err()
			return
		}
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var chunk ndjsonChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			// Skip malformed lines rather than abort the stream.
			continue
		}
		if chunk.Message.Content != "" {
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			case tokens <- chunk.Message.Content:
			}
		}
		if chunk.Done {
			return
		}
	}
	if err := sc.Err(); err != nil {
		errs <- fmt.Errorf("ollama: read stream: %w", err)
	}
}

// buildRequest assembles the chat payload.
func (c *Client) buildRequest(req domain.RewriteRequest) map[string]any {
	return map[string]any{
		"model":  c.model,
		"stream": true,
		"messages": []map[string]string{
			{"role": "system", "content": c.systemFmt(req.Tone())},
			{"role": "user", "content": req.Text()},
		},
	}
}

func readBriefly(r io.Reader) string {
	var b bytes.Buffer
	const max = 2048
	_, _ = io.Copy(&b, io.LimitReader(r, max))
	return b.String()
}

// ndjsonChunk is the subset of an Ollama /api/chat NDJSON line we use.
type ndjsonChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// DefaultSystemPrompt mirrors the other providers (Rule #1).
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
