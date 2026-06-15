package anthropic_test

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
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/anthropic"
)

// Compile-time proof the adapter satisfies the blocking-provider port.
var _ aicall.Completer = (*anthropic.Client)(nil)

func TestClient_Complete_NonStreaming(t *testing.T) {
	t.Parallel()
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-key", r.Header.Get("x-api-key"))
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"  hi there  "}]}`))
	}))
	defer srv.Close()

	c := anthropic.New(uuid.New(), "test-key", anthropic.WithEndpoint(srv.URL))
	text, ms, err := c.Complete(context.Background(), "sys", "user")
	require.NoError(t, err)
	require.Equal(t, "hi there", text) // trimmed
	require.GreaterOrEqual(t, ms, int64(0))
	require.Equal(t, false, gotBody["stream"])
}

func TestClient_Complete_PropagatesNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"overloaded"}`))
	}))
	defer srv.Close()

	c := anthropic.New(uuid.New(), "k", anthropic.WithEndpoint(srv.URL))
	_, _, err := c.Complete(context.Background(), "sys", "user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "status 500")
	require.NotContains(t, err.Error(), "overloaded") // no raw body leak
}
