package ollama_test

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

	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/ollama"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/provenance"
)

// streamNDJSON writes one NDJSON line per token then a final done:true line.
func streamNDJSON(w http.ResponseWriter, tokens []string) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	for _, t := range tokens {
		safe := strings.ReplaceAll(t, `"`, `\"`)
		fmt.Fprintf(w,
			"{\"model\":\"llama3.2\",\"message\":{\"role\":\"assistant\",\"content\":\"%s\"},\"done\":false}\n",
			safe)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	fmt.Fprint(w, "{\"model\":\"llama3.2\",\"done\":true,\"total_duration\":1234}\n")
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

func TestClient_StreamsContentTokens(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		streamNDJSON(w, []string{"Hello", " ", "world"})
	}))
	defer srv.Close()

	c := ollama.New(uuid.New(), ollama.WithEndpoint(srv.URL))
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, finalErr := drainTokens(t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"Hello", " ", "world"}, got)
}

func TestClient_StampsProvenance(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamNDJSON(w, []string{"Hello", " ", "world"})
	}))
	defer srv.Close()

	c := ollama.New(uuid.New(), ollama.WithEndpoint(srv.URL))
	ctx, p := provenance.NewContext(context.Background())
	tokens, errs := c.Stream(ctx, mustReq(t))
	_, finalErr := drainTokens(t, tokens, errs)
	require.NoError(t, finalErr)
	model, ptype := p.Read()
	require.Equal(t, "llama3.2", model)
	require.Equal(t, "ollama", ptype)
}

func TestClient_PropagatesNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"model not found"}`)
	}))
	defer srv.Close()
	c := ollama.New(uuid.New(), ollama.WithEndpoint(srv.URL))
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	_, finalErr := drainTokens(t, tokens, errs)
	require.Error(t, finalErr)
	require.Contains(t, finalErr.Error(), "503")
}

func TestClient_StopsOnDoneTrue(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{\"message\":{\"content\":\"first\"},\"done\":false}\n")
		fmt.Fprint(w, "{\"message\":{\"content\":\"\"},\"done\":true}\n")
		// Should NOT be consumed:
		fmt.Fprint(w, "{\"message\":{\"content\":\"after-done\"},\"done\":false}\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()
	c := ollama.New(uuid.New(), ollama.WithEndpoint(srv.URL))
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, finalErr := drainTokens(t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"first"}, got)
}

func TestClient_IgnoresMalformedLines(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "not-json\n")
		fmt.Fprint(w, "{\"message\":{\"content\":\"valid\"},\"done\":false}\n")
		fmt.Fprint(w, "{broken\n")
		fmt.Fprint(w, "{\"done\":true}\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()
	c := ollama.New(uuid.New(), ollama.WithEndpoint(srv.URL))
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
	c := ollama.New(uuid.New(), ollama.WithEndpoint(srv.URL))
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
	c := ollama.New(id)
	require.Equal(t, id, c.ID())
	require.Equal(t, "ollama", c.Name())
}
