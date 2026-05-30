package anthropic_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/adapter/anthropic"
	"github.com/tannpv/draftright-rewrite/internal/domain"
)

// streamDeltas writes one content_block_delta SSE frame per token —
// the slice of Anthropic Messages SSE we actually care about.
func streamDeltas(w http.ResponseWriter, tokens []string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
	for _, t := range tokens {
		safe := strings.ReplaceAll(t, `"`, `\"`)
		fmt.Fprintf(w,
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"%s\"}}\n\n",
			safe)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
}

func mustReq(t *testing.T) domain.RewriteRequest {
	t.Helper()
	r, err := domain.NewRewriteRequest("hi", "polished", "")
	require.NoError(t, err)
	return r
}

func drainTokens(t *testing.T, tokens <-chan string, errs <-chan error) ([]string, error) {
	t.Helper()
	var got []string
	deadline := time.After(3 * time.Second)
	for {
		select {
		case tok, ok := <-tokens:
			if !ok {
				select {
				case e := <-errs:
					return got, e
				default:
					return got, nil
				}
			}
			got = append(got, tok)
		case <-deadline:
			t.Fatal("drain timed out")
			return got, errors.New("timeout")
		}
	}
}

func TestClient_StreamsTextDeltas(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-key", r.Header.Get("x-api-key"))
		require.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		streamDeltas(w, []string{"Hello", " ", "world"})
	}))
	defer srv.Close()

	c := anthropic.New(uuid.New(), "test-key",
		anthropic.WithEndpoint(srv.URL))
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, finalErr := drainTokens(t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"Hello", " ", "world"}, got)
}

func TestClient_PropagatesNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid api key"}`)
	}))
	defer srv.Close()

	c := anthropic.New(uuid.New(), "wrong", anthropic.WithEndpoint(srv.URL))
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	_, finalErr := drainTokens(t, tokens, errs)
	require.Error(t, finalErr)
	require.Contains(t, finalErr.Error(), "401")
}

func TestClient_IgnoresOtherEvents(t *testing.T) {
	t.Parallel()
	// content_block_start, message_delta, ping events must NOT
	// produce tokens — only content_block_delta does.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
		fmt.Fprint(w, "event: ping\ndata: {}\n\n")
		fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"valid\"}}\n\n")
		fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\"}\n\n")
		fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()
	c := anthropic.New(uuid.New(), "k", anthropic.WithEndpoint(srv.URL))
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, finalErr := drainTokens(t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"valid"}, got)
}

func TestClient_ContextCancellation(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := anthropic.New(uuid.New(), "k", anthropic.WithEndpoint(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	tokens, errs := c.Stream(ctx, mustReq(t))
	cancel()
	for range tokens {
	}
	select {
	case <-errs:
	case <-time.After(2 * time.Second):
		t.Fatal("errs did not close after cancel")
	}
}

func TestClient_IDAndName(t *testing.T) {
	id := uuid.New()
	c := anthropic.New(id, "k")
	require.Equal(t, id, c.ID())
	require.Equal(t, "anthropic", c.Name())
}
