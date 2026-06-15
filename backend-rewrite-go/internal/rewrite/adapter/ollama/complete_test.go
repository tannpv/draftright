package ollama_test

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
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/ollama"
)

// Compile-time proof the adapter satisfies the blocking-provider port.
var _ aicall.Completer = (*ollama.Client)(nil)

func TestClient_Complete_NonStreaming(t *testing.T) {
	t.Parallel()
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"  local reply  "},"done":true}`))
	}))
	defer srv.Close()

	c := ollama.New(uuid.New(), ollama.WithEndpoint(srv.URL))
	text, ms, err := c.Complete(context.Background(), "sys", "user")
	require.NoError(t, err)
	require.Equal(t, "local reply", text) // trimmed
	require.GreaterOrEqual(t, ms, int64(0))
	require.Equal(t, false, gotBody["stream"])
}

func TestClient_Complete_PropagatesNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`model not loaded`))
	}))
	defer srv.Close()

	c := ollama.New(uuid.New(), ollama.WithEndpoint(srv.URL))
	_, _, err := c.Complete(context.Background(), "sys", "user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "status 503")
	require.NotContains(t, err.Error(), "not loaded") // no raw body leak
}
