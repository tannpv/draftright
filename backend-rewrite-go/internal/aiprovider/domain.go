package aiprovider

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// ErrNotFound is returned when an ai_provider row does not exist.
var ErrNotFound = errors.New("ai provider not found")

// AiProvider mirrors the ai_providers row. api_key is returned in plaintext
// on every read (Node parity). Field order matches
// src/ai-providers/entities/ai-provider.entity.ts exactly.
//
// Step 1 finding: the Node entity declares
//
//	@Column({ type: 'decimal', precision: 3, scale: 2, default: 0.3 })
//	temperature: number;
//
// with NO `transformer` (no ColumnNumericTransformer) — confirmed by reading
// the entity. TypeORM returns `decimal` columns as JS *strings* unless a
// numeric transformer is declared, so temperature serializes as a JSON string
// like "0.30". For byte-identical parity we model it as a Go string, not a
// float64.
type AiProvider struct {
	ID          string
	Name        string
	Type        string
	EndpointURL string
	APIKey      string
	Model       string
	Temperature string // decimal(3,2), no transformer → JSON string ("0.30"), see Step 1
	IsDefault   bool
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (p AiProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		EndpointURL string `json:"endpoint_url"`
		APIKey      string `json:"api_key"`
		Model       string `json:"model"`
		Temperature string `json:"temperature"`
		IsDefault   bool   `json:"is_default"`
		IsActive    bool   `json:"is_active"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}{
		ID: p.ID, Name: p.Name, Type: p.Type, EndpointURL: p.EndpointURL,
		APIKey: p.APIKey, Model: p.Model, Temperature: p.Temperature,
		IsDefault: p.IsDefault, IsActive: p.IsActive,
		CreatedAt: shared.ISOMillis(p.CreatedAt), UpdatedAt: shared.ISOMillis(p.UpdatedAt),
	})
}
