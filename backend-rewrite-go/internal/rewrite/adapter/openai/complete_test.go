package openai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/aicall"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/openai"
)

// Compile-time proof the adapter satisfies the blocking-provider port.
var _ aicall.Completer = (*openai.Client)(nil)

func TestClient_Complete_NonStreaming(t *testing.T) {
	t.Parallel()
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  hello world  "}}]}`))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "test-key", openai.WithEndpoint(srv.URL))
	text, ms, err := c.Complete(context.Background(), "sys", "user")
	require.NoError(t, err)
	require.Equal(t, "hello world", text) // trimmed
	require.GreaterOrEqual(t, ms, int64(0))
	_, hasStream := gotBody["stream"]
	require.False(t, hasStream, "Node buildBody omits the stream key")
}

func TestClient_Complete_OmitsAuthWhenNoKey(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasAuth := r.Header["Authorization"]
		require.False(t, hasAuth, "no Authorization header when api key is empty")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "", openai.WithEndpoint(srv.URL))
	_, _, err := c.Complete(context.Background(), "sys", "user")
	require.NoError(t, err)
}

func TestClient_Complete_PropagatesNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limit"}`))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "k", openai.WithEndpoint(srv.URL))
	_, _, err := c.Complete(context.Background(), "sys", "user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "status 429")
	require.NotContains(t, err.Error(), "rate limit") // no raw body leak
}

func TestComplete_BodyParity_NonGpt5(t *testing.T) {
	t.Parallel()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "k",
		openai.WithEndpoint(srv.URL),
		openai.WithModel("gpt-4o-mini"),
		openai.WithTemperature("0.30"),
	)
	_, _, err := c.Complete(context.Background(), "SYS", "USER")
	require.NoError(t, err)

	_, hasStream := got["stream"]
	require.False(t, hasStream, "Node buildBody omits the stream key")
	require.Equal(t, "gpt-4o-mini", got["model"])
	require.InDelta(t, 0.3, got["temperature"], 1e-9)
	_, hasReasoning := got["reasoning_effort"]
	require.False(t, hasReasoning)
	msgs := got["messages"].([]any)
	require.Len(t, msgs, 2)
	require.Equal(t, map[string]any{"role": "system", "content": "SYS"}, msgs[0])
	require.Equal(t, map[string]any{"role": "user", "content": "USER"}, msgs[1])
}

func TestComplete_BodyParity_Gpt5(t *testing.T) {
	t.Parallel()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "k",
		openai.WithEndpoint(srv.URL),
		openai.WithModel("gpt-5-nano"),
		openai.WithTemperature("0.30"),
	)
	_, _, err := c.Complete(context.Background(), "SYS", "USER")
	require.NoError(t, err)

	require.Equal(t, "minimal", got["reasoning_effort"])
	_, hasTemp := got["temperature"]
	require.False(t, hasTemp, "gpt-5 omits temperature")
}

func TestComplete_BodyParity_OllamaIsLocal(t *testing.T) {
	t.Parallel()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "",
		openai.WithEndpoint(srv.URL),
		openai.WithModel("llama3.2"),
		openai.WithTemperature("0.30"),
		openai.WithLocalLayout(true),
	)
	_, _, err := c.Complete(context.Background(), "SYS", "USER")
	require.NoError(t, err)

	msgs := got["messages"].([]any)
	require.Len(t, msgs, 2)
	sys := msgs[0].(map[string]any)
	require.Equal(t, "system", sys["role"])
	require.Contains(t, sys["content"], "You are a writing assistant.")
	require.Contains(t, sys["content"], "SAME LANGUAGE")
	usr := msgs[1].(map[string]any)
	require.Equal(t, "user", usr["role"])
	require.Equal(t,
		"SYS\n\nIMPORTANT: Your response MUST be in the same language as the text below.\n\nUser input:\n\"USER\"\n\nOutput:",
		usr["content"])
	require.InDelta(t, 0.3, got["temperature"], 1e-9)
}
