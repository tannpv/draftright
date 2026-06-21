package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// localSystemPrompt is Node's fixed isLocal (ollama) system message —
// openai.strategy.ts buildBody. Kept byte-identical for wire parity.
const localSystemPrompt = "You are a writing assistant. If the user provides existing text, rewrite it as instructed. If the user provides a request or instruction, generate the requested content. You ONLY output the result text. Never answer questions about yourself, never explain your process, never add commentary. CRITICAL: You must reply in the SAME LANGUAGE as the input text. If the input is Vietnamese, reply in Vietnamese. If French, reply in French. Never switch to English unless the input is in English."

// buildBody assembles the chat-completions request body byte-identically to
// Node OpenAiStrategy.buildBody: isLocal (ollama) message layout, gpt-5
// reasoning_effort vs temperature, no `stream` key.
func (c *Client) buildBody(system, user string) map[string]any {
	var messages []map[string]string
	if c.isLocal {
		messages = []map[string]string{
			{"role": "system", "content": localSystemPrompt},
			{"role": "user", "content": system +
				"\n\nIMPORTANT: Your response MUST be in the same language as the text below.\n\nUser input:\n\"" +
				user + "\"\n\nOutput:"},
		}
	} else {
		messages = []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		}
	}

	body := map[string]any{
		"model":    c.model,
		"messages": messages,
	}
	if strings.HasPrefix(c.model, "gpt-5") {
		body["reasoning_effort"] = "minimal"
	} else {
		temp, err := strconv.ParseFloat(c.temperature, 64)
		if err != nil {
			temp = 0
		}
		body["temperature"] = temp
	}
	return body
}

// Complete issues a blocking (non-streaming) chat-completions call and
// returns the full assistant text + round-trip milliseconds. Mirrors
// Node's OpenAiStrategy.call: stream omitted/false, parse
// choices[0].message.content (trimmed).
//
// Unlike Stream, the system + user prompts are passed verbatim — the
// blocking extraction path supplies its own prompts, so no Tone →
// systemFmt mapping happens here.
func (c *Client) Complete(ctx context.Context, system, user string) (string, int64, error) {
	start := time.Now()
	body, err := json.Marshal(c.buildBody(system, user))
	if err != nil {
		return "", time.Since(start).Milliseconds(), fmt.Errorf("provider %s: marshal request", c.Name())
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", time.Since(start).Milliseconds(), fmt.Errorf("provider %s: build request", c.Name())
	}
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
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
