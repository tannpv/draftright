package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Complete issues a blocking (non-streaming) chat-completions call and
// returns the full assistant text + round-trip milliseconds. Mirrors
// Node's OpenAiStrategy.call: stream omitted/false, parse
// choices[0].message.content (trimmed).
//
// Unlike Stream, the system + user prompts are passed verbatim — the
// blocking extraction path supplies its own prompts, so no Tone →
// systemFmt mapping happens here.
func (c *Client) Complete(ctx context.Context, system, user string) (string, int64, error) {
	body, err := json.Marshal(map[string]any{
		"model":  c.model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	if err != nil {
		return "", 0, fmt.Errorf("provider %s: marshal request", c.Name())
	}

	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", time.Since(start).Milliseconds(), fmt.Errorf("provider %s: build request", c.Name())
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
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
	if len(parsed.Choices) > 0 {
		text = strings.TrimSpace(parsed.Choices[0].Message.Content)
	}
	return text, ms, nil
}

// completeResponse is the subset of OpenAI's non-streaming response shape.
type completeResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
