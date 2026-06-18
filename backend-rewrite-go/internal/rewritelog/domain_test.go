package rewritelog_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tannpv/draftright-rewrite/internal/rewritelog"
)

func TestRewriteLog_JSONFieldOrder(t *testing.T) {
	l := rewritelog.RewriteLog{
		ID: "id1", Tone: "polished", InputText: "in", OutputText: "out",
		Model: "llama3.2", ProviderType: "ollama", ResponseTimeMs: 12,
		Quality: "pending", CreatedAt: time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(l)
	require.NoError(t, err)
	require.Equal(t, `{"id":"id1","tone":"polished","input_text":"in","output_text":"out","model":"llama3.2","provider_type":"ollama","response_time_ms":12,"quality":"pending","created_at":"2026-06-18T12:00:00.000Z"}`, string(b))
}
