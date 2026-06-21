package openai_test

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

	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/openai"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/provenance"
)

// streamChunks formats the given content tokens as the SSE chunks the
// real OpenAI API emits: "data: {…}\n\n" plus a terminal "data: [DONE]"
// line. Mirrors what we have to parse in production.
func streamChunks(w http.ResponseWriter, tokens []string, withDone bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	for _, t := range tokens {
		// Escape to fit inside JSON; tests use short strings so this
		// is sufficient. Use json.Marshal in production-grade fakes.
		safe := strings.ReplaceAll(t, `"`, `\"`)
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\n", safe)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	if withDone {
		fmt.Fprint(w, "data: [DONE]\n\n")
	}
}

func mustReq(t *testing.T) domain.RewriteRequest {
	t.Helper()
	r, err := domain.NewRewriteRequest("hi", "polished", "")
	require.NoError(t, err)
	return r
}

// drainTokens collects every chunk until tokens closes, plus the
// terminal error. Bounded by a generous timeout.
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

func TestClient_StreamsTokensFromHttptest(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		streamChunks(w, []string{"Hello", " ", "world"}, true)
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "test-key",
		openai.WithEndpoint(srv.URL),
		openai.WithModel("gpt-4o-mini"))
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, finalErr := drainTokens(t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"Hello", " ", "world"}, got)
}

func TestClient_StampsProvenance(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamChunks(w, []string{"Hello", " ", "world"}, true)
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "test-key",
		openai.WithEndpoint(srv.URL),
		openai.WithModel("gpt-4o-mini"))
	ctx, p := provenance.NewContext(context.Background())
	tokens, errs := c.Stream(ctx, mustReq(t))
	_, finalErr := drainTokens(t, tokens, errs)
	require.NoError(t, finalErr)
	model, ptype := p.Read()
	require.Equal(t, "gpt-4o-mini", model)
	require.Equal(t, "openai", ptype)
}

func TestClient_PropagatesNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"rate limit"}`)
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "k", openai.WithEndpoint(srv.URL))
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	_, finalErr := drainTokens(t, tokens, errs)
	require.Error(t, finalErr)
	require.Contains(t, finalErr.Error(), "429")
}

func TestClient_StopsOnDONE(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamChunks(w, []string{"only-token"}, true /* withDone */)
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "k", openai.WithEndpoint(srv.URL))
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, finalErr := drainTokens(t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"only-token"}, got)
}

func TestClient_IgnoresMalformedChunks(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Mix valid + broken chunks; broken ones should be skipped.
		fmt.Fprint(w, "data: NOT-JSON\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"valid\"}}]}\n\n")
		fmt.Fprint(w, "data: {malformed\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()
	c := openai.New(uuid.New(), "k", openai.WithEndpoint(srv.URL))
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, finalErr := drainTokens(t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"valid"}, got)
}

func TestClient_ContextCancellation(t *testing.T) {
	t.Parallel()
	// Server hangs forever. Client cancellation must close the
	// stream + close both channels promptly.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := openai.New(uuid.New(), "k", openai.WithEndpoint(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	tokens, errs := c.Stream(ctx, mustReq(t))
	cancel()

	for range tokens {
	}
	select {
	case e := <-errs:
		if e != nil {
			require.True(t,
				errors.Is(e, context.Canceled) || strings.Contains(e.Error(), "context canceled"),
				"want context.Canceled, got %v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("errs did not close after cancel")
	}
}

func TestClient_IDAndName(t *testing.T) {
	id := uuid.New()
	c := openai.New(id, "k")
	require.Equal(t, id, c.ID())
	require.Equal(t, "openai", c.Name())
}
