package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Complete issues a blocking (non-streaming) Messages API call and
// returns the full assistant text + round-trip milliseconds. Mirrors
// Node's AnthropicStrategy.call: stream omitted/false, parse
// content[0].text (trimmed).
//
// system + user prompts are passed verbatim — the blocking extraction
// path supplies its own prompts, so no Tone → systemFmt mapping here.
func (c *Client) Complete(ctx context.Context, system, user string) (string, int64, error) {
	start := time.Now()
	body, err := json.Marshal(map[string]any{
		"model":      c.model,
		"max_tokens": c.maxTokens,
		"stream":     false,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": user},
		},
	})
	if err != nil {
		return "", time.Since(start).Milliseconds(), fmt.Errorf("provider %s: marshal request", c.Name())
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", time.Since(start).Milliseconds(), fmt.Errorf("provider %s: build request", c.Name())
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", c.apiVersion)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", time.Since(start).Milliseconds(), fmt.Errorf("provider %s: do request", c.Name())
	}
	defer resp.Body.Close()

	ms := time.Since(start).Milliseconds()
	if resp.StatusCode/100 != 2 {
		return "", ms, fmt.Errorf("provider %s: status %d", c.Name(), resp.StatusCode)
	}

	var parsed completeResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", ms, fmt.Errorf("provider %s: decode response", c.Name())
	}
	var text string
	if len(parsed.Content) > 0 {
		text = strings.TrimSpace(parsed.Content[0].Text)
	}
	return text, ms, nil
}

// completeResponse is the subset of Anthropic's non-streaming response.
type completeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}
