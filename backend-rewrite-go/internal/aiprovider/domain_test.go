package aiprovider

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAiProvider_JSONKeyOrder(t *testing.T) {
	key := "sk-proj-abcd1234wxyz"
	p := AiProvider{
		ID: "11111111-1111-1111-1111-111111111111", Name: "OpenAI", Type: "openai",
		EndpointURL: "https://api.openai.com/v1", APIKey: key, Model: "gpt-4o-mini",
		Temperature: "0.30", IsDefault: true, IsActive: true,
		CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	order := []string{"id", "name", "type", "endpoint_url", "api_key", "model",
		"temperature", "is_default", "is_active", "created_at", "updated_at"}
	prev := -1
	for _, k := range order {
		idx := strings.Index(string(raw), `"`+k+`"`)
		if idx <= prev {
			t.Fatalf("key %q out of order: %s", k, raw)
		}
		prev = idx
	}
	if !strings.Contains(string(raw), `"api_key":"sk-…wxyz"`) {
		t.Fatalf("api_key must be masked (#29): %s", raw)
	}
	// decimal(3,2) has no TypeORM numeric transformer → Node serializes it as
	// a JSON string like "0.30" (parity).
	if !strings.Contains(string(raw), `"temperature":"0.30"`) {
		t.Fatalf("temperature must be decimal-as-string: %s", raw)
	}
	if !strings.Contains(string(raw), `"created_at":"1970-01-01T00:00:00.000Z"`) {
		t.Fatalf("timestamp not ISO-millis: %s", raw)
	}
}

// TestAiProvider_MasksAPIKey — #29: the raw api_key must never appear in the
// serialized response; only the first3…last4 mask. Mirrors Node maskProvider.
func TestAiProvider_MasksAPIKey(t *testing.T) {
	p := AiProvider{ID: "p1", Name: "OpenAI", Type: "openai", APIKey: "sk-proj-abcd1234wxyz", Model: "gpt-4o"}
	b, _ := json.Marshal(p)
	s := string(b)
	if strings.Contains(s, "sk-proj-abcd1234wxyz") {
		t.Errorf("leaks raw api_key: %s", s)
	}
	if !strings.Contains(s, `"api_key":"sk-…wxyz"`) {
		t.Errorf("expected masked api_key, got: %s", s)
	}
}
