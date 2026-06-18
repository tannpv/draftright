package rewritelog

// usecase.go — Service (admin training-data use case).
// Mirrors NestJS RewriteLogService (src/rewrite/rewrite-log.service.ts) +
// the admin.controller.ts training-data handlers.
//
// Node parity notes:
//   - Stats: count() + 3 × count({where:{quality}}) → {total,pending,approved,rejected}
//   - ListPending: findAndCount({where,order,skip,take}) → {logs,total}
//   - Review: update(id,{quality}) — NO validation of quality value (Node
//     accepts any string; validation omitted intentionally for parity)
//   - ExportJSONL: approved logs → JSONL. system template =
//     "Rewrite the following text in a <tone> tone. Return only the rewritten text."
//     Node uses JSON.stringify (no HTML escaping of <>&); we disable it here.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Repo is the consumer-side persistence port for the training-data use case.
// Satisfied by *PgRepo (repo_pg.go). Declared here (consumer side) per
// CLAUDE.md rule: interfaces belong to the consumer, kept to the exact width
// needed.
type Repo interface {
	Count(ctx context.Context) (int, error)
	CountByQuality(ctx context.Context) (pending, approved, rejected int, err error)
	FindPending(ctx context.Context, page, limit int) ([]RewriteLog, int, error)
	UpdateQuality(ctx context.Context, id, quality string) error
	FindApprovedAsc(ctx context.Context) ([]RewriteLog, error)
}

// StatsResult mirrors the JSON body of GET /admin/training-data/stats.
// Node: { total, pending, approved, rejected }
type StatsResult struct {
	Total    int `json:"total"`
	Pending  int `json:"pending"`
	Approved int `json:"approved"`
	Rejected int `json:"rejected"`
}

// Service is the admin training-data use case, parity with Node's
// RewriteLogService + admin.controller.ts training-data handlers.
type Service struct {
	repo Repo
}

// NewService wires the repo into the service.
func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

// Stats returns the total + per-quality counts.
// Mirrors Node: count() + countByQuality() → { total, pending, approved, rejected }.
func (s *Service) Stats(ctx context.Context) (StatsResult, error) {
	total, err := s.repo.Count(ctx)
	if err != nil {
		return StatsResult{}, err
	}
	pending, approved, rejected, err := s.repo.CountByQuality(ctx)
	if err != nil {
		return StatsResult{}, err
	}
	return StatsResult{
		Total:    total,
		Pending:  pending,
		Approved: approved,
		Rejected: rejected,
	}, nil
}

// ListPending returns the pending-quality logs (newest-first) for page/limit,
// plus the total pending count.
// Mirrors Node: findPending(page, limit) → { logs, total }.
func (s *Service) ListPending(ctx context.Context, page, limit int) ([]RewriteLog, int, error) {
	return s.repo.FindPending(ctx, page, limit)
}

// Review sets the quality field on a single log.
// NO quality validation — Node's updateQuality accepts any string.
// Mirrors Node: rewriteLogRepo.update(id, { quality }).
func (s *Service) Review(ctx context.Context, id, quality string) error {
	return s.repo.UpdateQuality(ctx, id, quality)
}

// ExportJSONL returns all approved logs as a JSONL string (one JSON object per
// line, lines joined by "\n"). Empty approved set → "".
//
// Each line shape (byte-identical to Node's JSON.stringify output):
//
//	{"messages":[
//	  {"role":"system","content":"Rewrite the following text in a <tone> tone. Return only the rewritten text."},
//	  {"role":"user","content":"<input_text>"},
//	  {"role":"assistant","content":"<output_text>"}
//	]}
//
// IMPORTANT: Go's json.Marshal escapes <, >, & by default (HTML safety).
// Node's JSON.stringify does NOT escape these characters. We use a
// json.Encoder with SetEscapeHTML(false) to match Node byte-for-byte.
func (s *Service) ExportJSONL(ctx context.Context) (string, error) {
	logs, err := s.repo.FindApprovedAsc(ctx)
	if err != nil {
		return "", err
	}
	if len(logs) == 0 {
		return "", nil
	}

	lines := make([]string, 0, len(logs))
	for _, log := range logs {
		line, err := marshalJSONLLine(log)
		if err != nil {
			return "", err
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), nil
}

// jsonlMessage is a single chat message in the JSONL training-data format.
type jsonlMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// jsonlRecord is the top-level shape of each JSONL line.
type jsonlRecord struct {
	Messages []jsonlMessage `json:"messages"`
}

// marshalJSONLLine encodes one RewriteLog as the JSONL line string.
// Uses a json.Encoder with SetEscapeHTML(false) so <, >, & are NOT HTML-escaped,
// matching Node's JSON.stringify behaviour. Encoder.Encode appends a trailing
// '\n' — we trim it before returning.
func marshalJSONLLine(log RewriteLog) (string, error) {
	rec := jsonlRecord{
		Messages: []jsonlMessage{
			{
				Role: "system",
				Content: fmt.Sprintf(
					"Rewrite the following text in a %s tone. Return only the rewritten text.",
					log.Tone,
				),
			},
			{Role: "user", Content: log.InputText},
			{Role: "assistant", Content: log.OutputText},
		},
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(rec); err != nil {
		return "", err
	}
	// Encoder.Encode always appends '\n'; trim it so callers control the join.
	return strings.TrimRight(buf.String(), "\n"), nil
}
