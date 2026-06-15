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
		raw, _ := io.ReadAll(r.Body)
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
	require.Equal(t, false, gotBody["stream"])
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
