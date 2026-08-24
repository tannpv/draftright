package aiprovider

import (
	"testing"

	"github.com/google/uuid"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/anthropic"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/ollama"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/openai"
)

// TestProviderNamesMatchTypes is the Rule #1 can't-merge guard (#204 finding #4):
// the rewrite adapters can't import this package (import cycle), so their Name()
// ids are declared separately from the ProviderType consts the completer factory
// dispatches on. This asserts the two agree — a rename on either side fails here
// instead of silently mis-routing (the factory's default branch would reject the
// provider at runtime).
func TestProviderNamesMatchTypes(t *testing.T) {
	cases := []struct {
		got  string
		want ProviderType
	}{
		{openai.New(uuid.Nil, "").Name(), ProviderOpenAI},
		{anthropic.New(uuid.Nil, "").Name(), ProviderAnthropic},
		{ollama.New(uuid.Nil).Name(), ProviderOllama},
	}
	for _, c := range cases {
		if c.got != string(c.want) {
			t.Errorf("adapter Name()=%q, ProviderType const=%q — they must stay in sync", c.got, c.want)
		}
	}
}
