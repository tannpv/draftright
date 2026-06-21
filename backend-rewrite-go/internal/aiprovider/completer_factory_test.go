package aiprovider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFactory_SatisfiesPort is a compile + runtime assertion that Factory{}
// satisfies the consumer-side CompleterFactory port (method is For).
func TestFactory_SatisfiesPort(t *testing.T) {
	var f CompleterFactory = Factory{}
	if f == nil {
		t.Fatal("Factory{} must satisfy CompleterFactory")
	}
}

// TestFactory_BuildsKnownTypes covers every dispatched provider type. Each must
// build a non-nil Completer with no error. No network call is made (we never
// call Complete) — only construction is asserted.
func TestFactory_BuildsKnownTypes(t *testing.T) {
	cases := []struct {
		name string
		typ  string
	}{
		{"openai", "openai"},
		{"anthropic", "anthropic"},
		{"ollama", "ollama"},
		{"custom", "custom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := AiProvider{
				ID:          "11111111-1111-1111-1111-111111111111",
				Name:        "My Provider",
				Type:        tc.typ,
				EndpointURL: "https://example.com/v1",
				APIKey:      "secret-key",
				Model:       "some-model",
			}
			c, err := Factory{}.For(p)
			if err != nil {
				t.Fatalf("For(%q) returned error: %v", tc.typ, err)
			}
			if c == nil {
				t.Fatalf("For(%q) returned nil Completer", tc.typ)
			}
		})
	}
}

func TestFactory_OllamaUsesOpenAIWire(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  ok  "}}]}`))
	}))
	defer srv.Close()

	p := AiProvider{
		ID:          "11111111-1111-1111-1111-111111111111",
		Name:        "Local Ollama",
		Type:        "ollama",
		EndpointURL: srv.URL + "/v1/chat/completions",
		APIKey:      "",
		Model:       "llama3.2",
	}
	c, err := Factory{}.For(p)
	require.NoError(t, err)
	require.NotNil(t, c)

	text, _, err := c.Complete(context.Background(), "sys", "user")
	require.NoError(t, err)
	require.Equal(t, "ok", text)

	require.Equal(t, "/v1/chat/completions", gotPath)
	msgs, ok := gotBody["messages"].([]any)
	require.True(t, ok, "body must carry a messages array")
	require.Len(t, msgs, 2)
	require.Equal(t, "system", msgs[0].(map[string]any)["role"])
	require.Equal(t, "user", msgs[1].(map[string]any)["role"])
	require.Equal(t, "llama3.2", gotBody["model"])
}

// TestFactory_BuildsWhenEndpointEmpty: openai/anthropic must still build when
// EndpointURL is empty (the WithEndpoint option is skipped, defaults apply).
func TestFactory_BuildsWhenEndpointEmpty(t *testing.T) {
	for _, typ := range []string{"openai", "anthropic"} {
		p := AiProvider{
			ID:     "11111111-1111-1111-1111-111111111111",
			Name:   "My Provider",
			Type:   typ,
			Model:  "some-model",
			APIKey: "secret-key",
		}
		c, err := Factory{}.For(p)
		if err != nil || c == nil {
			t.Fatalf("For(%q) with empty endpoint: completer=%v err=%v", typ, c, err)
		}
	}
}

// TestFactory_BuildsWhenIDMalformed: a non-uuid ID must not error — the factory
// mints a fresh uuid (mirrors resolveProviderID mint-on-malformed policy).
func TestFactory_BuildsWhenIDMalformed(t *testing.T) {
	p := AiProvider{
		ID:     "not-a-uuid",
		Name:   "My Provider",
		Type:   "openai",
		Model:  "some-model",
		APIKey: "secret-key",
	}
	c, err := Factory{}.For(p)
	if err != nil || c == nil {
		t.Fatalf("For with malformed id: completer=%v err=%v", c, err)
	}
}

// TestFactory_UnknownTypeError: an unregistered type yields a nil Completer and
// an error whose message is byte-identical to Node's registry.pick() throw.
func TestFactory_UnknownTypeError(t *testing.T) {
	p := AiProvider{Type: "frobnicator", ID: "abc-123", Name: "My Prov"}
	c, err := Factory{}.For(p)
	if c != nil {
		t.Fatalf("expected nil Completer for unknown type, got %v", c)
	}
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
	want := `No provider strategy registered for type "frobnicator" (provider id=abc-123, name=My Prov)`
	if err.Error() != want {
		t.Fatalf("error mismatch:\n got: %q\nwant: %q", err.Error(), want)
	}
}
