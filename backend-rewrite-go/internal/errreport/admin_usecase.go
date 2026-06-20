package errreport

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
)

// adminRepo is the persistence port for admin error operations (B1).
type adminRepo interface {
	AdminList(ctx context.Context, f AdminListFilter) ([]ErrorReportEntity, int, error)
	AdminGet(ctx context.Context, id string) (ErrorReportEntity, error)
	AdminDelete(ctx context.Context, id string) (bool, error)
	AdminSetStatusRaw(ctx context.Context, id string, statusText *string, setResolved bool, resolvedAt *time.Time, resolvedBy *string) (ErrorReportEntity, error)
	AdminSetFixProposal(ctx context.Context, id string, proposal string, status int) (ErrorReportEntity, error)
	AdminFixCandidates(ctx context.Context, limit int32) ([]string, error)
}

// fixProposer resolves the default AI provider and runs a completion.
// Satisfied by *aiprovider.Service.Propose.
type fixProposer interface {
	Propose(ctx context.Context, system, user string) (string, error)
}

// ErrNotFoundMsg is the exact Node message for a missing error (400, NOT 404).
const ErrNotFoundMsg = "not found"

// AdminService implements the admin error-triage use cases (E1–E5 logic).
type AdminService struct {
	repo     adminRepo
	proposer fixProposer
	now      func() time.Time
}

// NewAdminService wires the repo, AI fix proposer, and clock.
func NewAdminService(repo adminRepo, proposer fixProposer, now func() time.Time) *AdminService {
	return &AdminService{repo: repo, proposer: proposer, now: now}
}

// List returns the filtered page of error rows + the total count.
func (s *AdminService) List(ctx context.Context, f AdminListFilter) ([]ErrorReportEntity, int, error) {
	return s.repo.AdminList(ctx, f)
}

// Get loads one error row, returning ErrNotFound when absent.
func (s *AdminService) Get(ctx context.Context, id string) (ErrorReportEntity, error) {
	return s.repo.AdminGet(ctx, id)
}

// Delete hard-deletes by id; returns whether a row was removed (idempotent).
func (s *AdminService) Delete(ctx context.Context, id string) (bool, error) {
	return s.repo.AdminDelete(ctx, id)
}

// SetStatusRaw loads the row (404 → ErrNotFound), then persists the RAW status
// value exactly as Node's ErrorsService.setStatus does: Node binds body.status
// (any JSON type) straight to the int4 column and lets Postgres coerce — so a
// non-numeric string or json null produces a Postgres 500, not a Go-side 400.
//
// resolved_at/resolved_by are stamped only when the raw value is the JSON
// *number* 4 (RESOLVED) or 5 (IGNORED) — Node uses strict `=== 4/5`, so a JSON
// string "4" does NOT resolve. Other values preserve the stored resolved_*
// (the SQL CASE keys off setResolved). The AdminGet-first ordering matches
// Node's findOne-before-save: an absent id with a bad status → 400 "not found",
// never a 500. statusRaw==nil means the caller already rejected absent status.
func (s *AdminService) SetStatusRaw(ctx context.Context, id string, statusRaw json.RawMessage, resolvedBy string) (ErrorReportEntity, error) {
	if _, err := s.repo.AdminGet(ctx, id); err != nil {
		return ErrorReportEntity{}, err
	}
	statusText, setResolved := renderStatus(statusRaw)
	var resolvedAt *time.Time
	var rb *string
	if setResolved {
		now := s.now()
		resolvedAt = &now
		v := resolvedBy
		if v == "" {
			v = "admin"
		}
		rb = &v
	}
	return s.repo.AdminSetStatusRaw(ctx, id, statusText, setResolved, resolvedAt, rb)
}

// renderStatus converts the raw JSON status value into the TEXT node-pg would
// bind to the int4 column, plus whether it is the resolving number 4/5.
//   - JSON null → nil (→ SQL NULL → not-null violation, Node parity)
//   - JSON number → JS String(n): integers without a decimal ("4"), others via
//     shortest round-trip ("2.5"); resolves iff == 4 or == 5
//   - JSON string → the unquoted value ("foo", "3") — never resolves
//   - JSON bool → "true"/"false"; object/array → raw text — both hit a PG error
func renderStatus(raw json.RawMessage) (text *string, resolveStatus bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		s := strings.TrimSpace(string(raw))
		return &s, false
	}
	switch vv := v.(type) {
	case nil:
		return nil, false
	case json.Number:
		s := vv.String()
		if f, ferr := vv.Float64(); ferr == nil {
			if f == math.Trunc(f) && f >= math.MinInt32 && f <= math.MaxInt32 {
				s = strconv.FormatInt(int64(f), 10)
			}
			return &s, f == 4 || f == 5
		}
		return &s, false
	case string:
		return &vv, false
	case bool:
		s := strconv.FormatBool(vv)
		return &s, false
	default:
		s := strings.TrimSpace(string(raw))
		return &s, false
	}
}

// SuggestFix loads the row (404 → ErrNotFound), builds the §3.2 prompt, runs
// the default AI provider, and persists the proposal with status=3 (FIX_PROPOSED).
// Provider errors (incl. ErrNoDefaultProvider) propagate. Mirrors
// ErrorsService.suggestFix.
func (s *AdminService) SuggestFix(ctx context.Context, id string) (ErrorReportEntity, error) {
	row, err := s.repo.AdminGet(ctx, id)
	if err != nil {
		return ErrorReportEntity{}, err
	}
	system, user := buildErrorPrompt(row)
	text, err := s.proposer.Propose(ctx, system, user)
	if err != nil {
		return ErrorReportEntity{}, err
	}
	return s.repo.AdminSetFixProposal(ctx, id, text, 3)
}

// AdminFixCandidates returns the ids of the top un-analyzed error groups for
// the hourly fix-proposal cron (cronSvc port). Delegates to the repo, which
// applies the Node selection (status=0, ai_fix_proposal IS NULL, count >= 2).
func (s *AdminService) AdminFixCandidates(ctx context.Context, limit int32) ([]string, error) {
	return s.repo.AdminFixCandidates(ctx, limit)
}

// errorSystemPrompt is transcribed byte-for-byte from
// src/errors/errors.service.ts suggestFix() systemPrompt.
const errorSystemPrompt = `You are a senior software engineer reviewing a production crash report. Analyze the error and propose a concrete fix.

Output format (concise; this is shown to a developer in a triage queue):

ROOT CAUSE
<one sentence — what went wrong, in plain language>

LIKELY FILE / FRAME
<the most likely file:line from the stack trace, or "unknown" if the stack is opaque>

PROPOSED FIX
<2-4 bullet points — what to change. Code snippets if obvious. No fluff.>

CONFIDENCE
<low | medium | high>

If the stack is genuinely insufficient to propose a fix, say so plainly and suggest what additional information would help (e.g., "need source for X", "need a reproduction").

Do not output anything outside this format. Do not pad. Do not apologize.`

// buildErrorPrompt assembles the §3.2 system + user prompts, transcribed
// byte-for-byte from errors.service.ts suggestFix(). The user block is joined
// with "\n" and bounded to 8000 UTF-16 code units (JS String.slice(0, 8000)).
func buildErrorPrompt(e ErrorReportEntity) (system, user string) {
	user = strings.Join([]string{
		"Platform: " + e.Platform,
		"App version: " + derefOr(e.AppVersion, "unknown"),
		"Severity: " + e.Severity,
		"Occurrences: " + strconv.Itoa(e.Count),
		"First seen: " + orUnknown(e.FirstSeenAt),
		"Last seen: " + orUnknown(e.LastSeenAt),
		"",
		"Error type: " + derefOr(e.ErrorType, "(none)"),
		"Message: " + derefOr(e.Message, "(none)"),
		"",
		"Stack trace:",
		stackOrEmpty(e.StackTrace),
		"",
		"Context: " + jsonIndent2(e.Context),
	}, "\n")
	return errorSystemPrompt, sliceUTF16(user, 8000)
}

// derefOr returns *p when non-nil and non-empty, else def. Mirrors the JS
// `x || 'default'` idiom (empty string is falsy in JS).
func derefOr(p *string, def string) string {
	if p == nil || *p == "" {
		return def
	}
	return *p
}

// orUnknown mirrors `row.x?.toISOString?.() ?? 'unknown'` — the entity already
// holds the ISO string, so an empty string (null column) maps to 'unknown'.
func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// stackOrEmpty mirrors `row.stack_trace || '(empty)'`.
func stackOrEmpty(p *string) string {
	if p == nil || *p == "" {
		return "(empty)"
	}
	return *p
}

// jsonIndent2 mirrors JSON.stringify(row.context || {}, null, 2): nil/empty
// context becomes "{}", otherwise the raw JSON is re-indented with 2 spaces.
func jsonIndent2(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return "{}"
	}
	return buf.String()
}
