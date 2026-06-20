package rewritelog

import (
	"encoding/json"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// RewriteLog is one row of the rewrite_logs table (admin training-data).
// MarshalJSON pins entity field order + ISOMillis created_at to match Node's
// serialized TypeORM entity byte-for-byte.
//
// Node entity column order (rewrite-log.entity.ts):
//
//	id, tone, input_text, output_text, model, provider_type,
//	response_time_ms, quality, created_at
type RewriteLog struct {
	ID             string
	Tone           string
	InputText      string
	OutputText     string
	Model          string
	ProviderType   string
	ResponseTimeMs int
	Quality        string
	CreatedAt      time.Time
}

// RewriteLogInput is the write side of rewrite_logs — the six fields the
// rewrite flow captures after a successful provider call. id, quality, and
// created_at are supplied by DB defaults (uuid, 'pending', now()), mirroring
// Node's rewriteLogRepo.save({ tone, input_text, output_text, model,
// provider_type, response_time_ms }).
type RewriteLogInput struct {
	Tone           string
	InputText      string
	OutputText     string
	Model          string
	ProviderType   string
	ResponseTimeMs int64
}

func (l RewriteLog) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID             string `json:"id"`
		Tone           string `json:"tone"`
		InputText      string `json:"input_text"`
		OutputText     string `json:"output_text"`
		Model          string `json:"model"`
		ProviderType   string `json:"provider_type"`
		ResponseTimeMs int    `json:"response_time_ms"`
		Quality        string `json:"quality"`
		CreatedAt      string `json:"created_at"`
	}
	return json.Marshal(wire{
		ID:             l.ID,
		Tone:           l.Tone,
		InputText:      l.InputText,
		OutputText:     l.OutputText,
		Model:          l.Model,
		ProviderType:   l.ProviderType,
		ResponseTimeMs: l.ResponseTimeMs,
		Quality:        l.Quality,
		CreatedAt:      shared.ISOMillis(l.CreatedAt),
	})
}
