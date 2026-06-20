package bugreports

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
)

// adminRepo is the persistence port for the bug-report admin operations
// (C-group). Satisfied by *AdminPgRepo.
type adminRepo interface {
	AdminList(ctx context.Context, f AdminListFilter) ([]BugReportEntity, int, error)
	AdminGet(ctx context.Context, id string) (BugReportEntity, error)
	AdminDelete(ctx context.Context, id string) (bool, error)
	AdminUpdate(ctx context.Context, id string, status string, adminNotes, title, targetPlatform *string, isPublic bool) (BugReportEntity, error)
	AdminSetFixProposal(ctx context.Context, id string, proposal string, proposedAt time.Time, status string) (BugReportEntity, error)
	AdminFixCandidates(ctx context.Context, limit int32) ([]string, error)
}

// fixProposer resolves the default AI provider and runs a completion.
// Satisfied by *aiprovider.Service.Propose (shared with errreport).
type fixProposer interface {
	Propose(ctx context.Context, system, user string) (string, error)
}

// Screenshot is the on-disk pointer returned by GetScreenshot (Node
// getScreenshotPath → { path, filename } | null).
type Screenshot struct {
	Path     string
	Filename string
}

// BugPatch is the validated PATCH /admin/bug-reports/:id body. Each pointer is
// nil when the JSON key was absent (Node `dto.x !== undefined`), so the merge
// touches only the provided columns. Status/Title/TargetPlatform validation
// runs in Update; the handler decodes the raw body into these optionals.
// BugPatch is one PATCH /admin/bug-reports/:id body. Node's handler uses
// `dto.x !== undefined` to decide whether to touch each field, so an explicit
// JSON `null` (present) behaves DIFFERENTLY from an absent key — and a Go
// `*T == nil` decode collapses the two. Each <Field>Set bool restores that
// distinction: true ⇔ the key was present (even as null), mirroring `!==
// undefined`. The value pointer is nil when the present value was null or a
// non-string (Node then validates it → 400, or for admin_notes persists NULL).
// See issue #39. is_public keeps plain pointer semantics (nil = absent): a
// present-null/non-bool is_public is a leaky DB-coerced edge left documented,
// not fabricated here.
type BugPatch struct {
	StatusSet         bool
	Status            *string
	AdminNotesSet     bool
	AdminNotes        *string
	TitleSet          bool
	Title             *string
	TargetPlatformSet bool
	TargetPlatform    *string
	IsPublic          *bool
}

// AdminService implements the bug-report admin-triage use cases (C-group).
type AdminService struct {
	repo     adminRepo
	proposer fixProposer
	now      func() time.Time
}

// NewAdminService wires the repo, AI fix proposer, and clock.
func NewAdminService(repo adminRepo, proposer fixProposer, now func() time.Time) *AdminService {
	return &AdminService{repo: repo, proposer: proposer, now: now}
}

// List returns the filtered page of bug rows + the total count.
func (s *AdminService) List(ctx context.Context, f AdminListFilter) ([]BugReportEntity, int, error) {
	return s.repo.AdminList(ctx, f)
}

// Get loads one bug row, returning ErrNotFound when absent (Node findById).
func (s *AdminService) Get(ctx context.Context, id string) (BugReportEntity, error) {
	return s.repo.AdminGet(ctx, id)
}

// Delete loads the row first (404 → ErrNotFound), then hard-deletes. Node's
// delete() does findById (which 404s) before repo.delete; the screenshot
// unlink is best-effort and handled by the OS layer — here the row delete
// alone is sufficient for parity (the 204 body is empty either way). The
// screenshot file removal happens in the handler/caller if needed; Node's
// best-effort unlink never affects the response.
func (s *AdminService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.AdminGet(ctx, id); err != nil {
		return err
	}
	if _, err := s.repo.AdminDelete(ctx, id); err != nil {
		return err
	}
	return nil
}

// GetScreenshot returns the on-disk screenshot pointer, or nil when the row
// has no screenshot. 404s (ErrNotFound) when the row is missing. Mirrors
// getScreenshotPath: filename falls back to path.basename(screenshot_path).
func (s *AdminService) GetScreenshot(ctx context.Context, id string) (*Screenshot, error) {
	row, err := s.repo.AdminGet(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.ScreenshotPath == nil || *row.ScreenshotPath == "" {
		return nil, nil
	}
	filename := ""
	if row.ScreenshotFilename != nil {
		filename = *row.ScreenshotFilename
	}
	if filename == "" {
		filename = filepath.Base(*row.ScreenshotPath)
	}
	return &Screenshot{Path: *row.ScreenshotPath, Filename: filename}, nil
}

// Update applies the per-field merge in Node's exact order (status,
// admin_notes, title, target_platform, is_public) with the verbatim validation
// messages, loads-then-saves (404 → ErrNotFound on the load), and returns the
// saved row. The merge resolves each absent field to the row's current value
// so AdminUpdate writes the full column set.
func (s *AdminService) Update(ctx context.Context, id string, p BugPatch) (BugReportEntity, error) {
	row, err := s.repo.AdminGet(ctx, id)
	if err != nil {
		return BugReportEntity{}, err
	}

	// Start from the current row values; apply only provided fields.
	status := row.Status
	adminNotes := row.AdminNotes
	title := row.Title
	targetPlatform := row.TargetPlatform
	isPublic := row.IsPublic

	// Each branch fires on PRESENCE (Node `!== undefined`), so an explicit null
	// — which arrives as Set=true with a nil value pointer — is validated, not
	// silently skipped. A present null/non-string status or target_platform is
	// "not in the allow-list" → 400 (matching Node's `includes(null)` → false);
	// a present null title trims to "" → 400; a present null admin_notes
	// persists SQL NULL.
	if p.StatusSet {
		if p.Status == nil || !bugStatusValid(*p.Status) {
			return BugReportEntity{}, &BugValidationError{Msg: "status must be one of: " + strings.Join(bugAllowedStatuses, ", ")}
		}
		status = *p.Status
	}
	if p.AdminNotesSet {
		adminNotes = p.AdminNotes
	}
	if p.TitleSet {
		t := ""
		if p.Title != nil {
			t = strings.TrimSpace(*p.Title)
		}
		if lenUTF16Bug(t) < 1 || lenUTF16Bug(t) > 80 {
			return BugReportEntity{}, &BugValidationError{Msg: "title is required (1-80 characters)"}
		}
		title = &t
	}
	if p.TargetPlatformSet {
		if p.TargetPlatform == nil || !bugTargetPlatformValid(*p.TargetPlatform) {
			return BugReportEntity{}, &BugValidationError{Msg: "target_platform must be one of: " + strings.Join(bugTargetPlatforms, ", ")}
		}
		tp := *p.TargetPlatform
		targetPlatform = &tp
	}
	if p.IsPublic != nil {
		isPublic = *p.IsPublic
	}

	return s.repo.AdminUpdate(ctx, id, status, adminNotes, title, targetPlatform, isPublic)
}

// SuggestFix loads the row (404 → ErrNotFound), builds the §3.3 prompt, runs
// the default AI provider, and persists the proposal with ai_fix_proposed_at =
// now and status → 'fix_proposed' ONLY when it was 'new'. Provider errors
// (incl. ErrNoDefaultProvider) propagate. Mirrors BugReportsService.suggestFix.
func (s *AdminService) SuggestFix(ctx context.Context, id string) (BugReportEntity, error) {
	row, err := s.repo.AdminGet(ctx, id)
	if err != nil {
		return BugReportEntity{}, err
	}
	system, user := buildBugPrompt(row)
	text, err := s.proposer.Propose(ctx, system, user)
	if err != nil {
		return BugReportEntity{}, err
	}
	newStatus := row.Status
	if row.Status == "new" {
		newStatus = "fix_proposed"
	}
	return s.repo.AdminSetFixProposal(ctx, id, text, s.now(), newStatus)
}

// AdminBugFixCandidates is the cron port method (matches cronSvc): newest
// un-analyzed bug ids, capped at limit (cron passes 5).
func (s *AdminService) AdminBugFixCandidates(ctx context.Context, limit int32) ([]string, error) {
	return s.repo.AdminFixCandidates(ctx, limit)
}

// BugValidationError is a 400 BadRequestException equivalent — the handler maps
// it to invalid-input with Msg. Mirrors Node's per-field BadRequestException.
type BugValidationError struct{ Msg string }

func (e *BugValidationError) Error() string { return e.Msg }

// bugStatusValid reports whether s ∈ ALLOWED_STATUSES.
func bugStatusValid(s string) bool {
	for _, v := range bugAllowedStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// bugTargetPlatformValid reports whether p ∈ TARGET_PLATFORMS.
func bugTargetPlatformValid(p string) bool {
	for _, v := range bugTargetPlatforms {
		if v == p {
			return true
		}
	}
	return false
}

// bugSystemPrompt is transcribed byte-for-byte from
// src/bug-reports/bug-reports.service.ts suggestFix() systemPrompt.
const bugSystemPrompt = `You are a senior software engineer reviewing a user-submitted bug report from a production app. The user described what's wrong in their own words; you have NO stack trace and possibly no reproduction steps. Propose a triage plan.

Output format (concise; this is shown to a developer in a triage queue):

LIKELY ROOT CAUSE
<one or two sentences — your best guess based on the description, platform, and version. Cite parts of the description that support it.>

LIKELY AREA OF CODE
<which module, screen, or feature this most likely lives in. "unknown" if too vague.>

PROPOSED FIX OR INVESTIGATION
<2-4 bullet points — either a concrete code change OR specific things the developer should check / questions to ask the user.>

CONFIDENCE
<low | medium | high>

REPRODUCTION QUESTIONS
<2-3 short questions to ask the user IF the report is too vague to act on. Skip this section if confidence is medium or high.>

Do not output anything outside this format. Do not pad. Do not apologize.`

// buildBugPrompt assembles the §3.3 system + user prompts, transcribed
// byte-for-byte from bug-reports.service.ts suggestFix(). The user block is
// joined with "\n" and bounded to 8000 UTF-16 code units (JS slice(0, 8000)).
func buildBugPrompt(b BugReportEntity) (system, user string) {
	hasScreenshot := "no"
	if b.ScreenshotPath != nil && *b.ScreenshotPath != "" {
		hasScreenshot = "yes"
	}
	user = strings.Join([]string{
		"Source: " + b.Source,
		"Platform / OS: " + bugOrUnknown(b.OsInfo),
		"App version: " + bugOrUnknown(b.AppVersion),
		"Submitted: " + bugCreatedOrUnknown(b.CreatedAt),
		"User email: " + bugOrAnonymous(b.UserEmail),
		"Has screenshot: " + hasScreenshot,
		"",
		"User's description:",
		b.Description,
		"",
		"Context: " + bugJSONIndent2(b.Context),
	}, "\n")
	return bugSystemPrompt, sliceUTF16Bug(user, 8000)
}

// bugOrUnknown mirrors `row.x || 'unknown'` (empty/nil → "unknown").
func bugOrUnknown(p *string) string {
	if p == nil || *p == "" {
		return "unknown"
	}
	return *p
}

// bugOrAnonymous mirrors `row.user_email || '(anonymous)'`.
func bugOrAnonymous(p *string) string {
	if p == nil || *p == "" {
		return "(anonymous)"
	}
	return *p
}

// bugCreatedOrUnknown mirrors `row.created_at?.toISOString?.() ?? 'unknown'`.
// The entity already holds the ISO-millis string, so an empty string (null
// column) maps to 'unknown'.
func bugCreatedOrUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// bugJSONIndent2 mirrors JSON.stringify(row.context || {}, null, 2): nil/empty
// context becomes "{}", otherwise the raw JSON is re-indented with 2 spaces.
func bugJSONIndent2(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return "{}"
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return "{}"
	}
	return buf.String()
}

// sliceUTF16Bug caps s to n UTF-16 code units, mirroring JS String.slice(0, n).
func sliceUTF16Bug(s string, n int) string {
	u := utf16.Encode([]rune(s))
	if len(u) <= n {
		return s
	}
	return string(utf16.Decode(u[:n]))
}

// lenUTF16Bug returns s length in UTF-16 code units (JS String.length), what
// the title 1-80 bound measures.
func lenUTF16Bug(s string) int {
	return len(utf16.Encode([]rune(s)))
}
