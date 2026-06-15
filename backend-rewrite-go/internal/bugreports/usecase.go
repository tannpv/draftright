package bugreports

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf16"
)

// ErrDescriptionRequired / ErrSourceRequired mirror the service-level
// BadRequestExceptions in BugReportsService.create (bug-reports.service.ts:71-76).
// These are a SECOND guard after the DTO ValidationPipe — they fire on a
// whitespace-only value that the DTO's @MinLength(1) (which counts spaces as
// length) would let through. Mapped to 400 invalid-input by the handler.
var (
	ErrDescriptionRequired = errors.New("description is required")
	ErrSourceRequired      = errors.New("source is required")
)

// CreateInput is the trimmed/validated text payload (post-DTO).
type CreateInput struct {
	Description string
	Source      string
	AppVersion  string // "" when absent
	OsInfo      string // "" when absent
	UserEmail   string // "" when absent
	Context     string // "" when absent; raw JSON-as-string from the form
	HasContext  bool   // distinguishes a supplied empty context from absent
}

// FilePart is the uploaded screenshot (nil when none).
type FilePart struct {
	Buffer       []byte
	OriginalName string
	Mimetype     string
}

// Repo is the consumer-side port the handler/service depend on.
type Repo interface {
	Insert(ctx context.Context, n NewRow) (Created, error)
	ResolveUserID(ctx context.Context, id *string) (*string, error)
}

// Service creates bug-report rows. Ports BugReportsService.create.
type Service struct {
	repo  Repo
	store *Storage
}

// NewService wires the repo + screenshot storage.
func NewService(repo Repo, store *Storage) *Service { return &Service{repo: repo, store: store} }

// Create ports BugReportsService.create (bug-reports.service.ts:62-118):
// trim+guard description/source, optionally persist the screenshot, parse the
// context (with a {raw:...} fallback), slice each field to its column width,
// resolve the user id, and insert. userID is the best-effort JWT subject ("" =
// anonymous). file is nil when no screenshot was uploaded.
func (s *Service) Create(ctx context.Context, in CreateInput, file *FilePart, userID string) (Created, error) {
	// Service-level required guards (after DTO validation). Node trims then
	// checks length == 0.
	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		return Created{}, ErrDescriptionRequired
	}
	src := strings.TrimSpace(in.Source)
	if src == "" {
		return Created{}, ErrSourceRequired
	}

	var screenshotPath, screenshotFilename *string
	if file != nil {
		path, filename, err := s.store.Save(file.Buffer, file.Mimetype, file.OriginalName)
		if err != nil {
			// Storage's extensionFor returns errUnsupportedMime for a mime
			// the service can't store — Node throws the same string here.
			return Created{}, err
		}
		screenshotPath = &path
		screenshotFilename = &filename
	}

	// context: JSON.parse with a {raw:<original>} fallback on parse failure.
	var contextBytes []byte
	if in.HasContext && in.Context != "" {
		if json.Valid([]byte(in.Context)) {
			contextBytes = []byte(in.Context)
		} else if fallback, err := json.Marshal(map[string]string{"raw": in.Context}); err == nil {
			contextBytes = fallback
		}
		// On the (practically impossible) marshal failure, contextBytes stays
		// nil → store NULL rather than invalid JSON.
	}

	// Field slicing to column widths (Node .slice() calls in create):
	//   source        → 50   description → trimmed, no slice
	//   app_version   → 50   os_info     → 100
	//   user_email    → 255
	row := NewRow{
		Source:             sliceUTF16(src, 50),
		Description:        desc,
		ScreenshotPath:     screenshotPath,
		ScreenshotFilename: screenshotFilename,
		AppVersion:         sliceOrNil(in.AppVersion, 50),
		OsInfo:             sliceOrNil(in.OsInfo, 100),
		UserEmail:          sliceOrNil(in.UserEmail, 255),
		Context:            contextBytes,
	}

	// resolveUserId: null the FK when the JWT outlives its user.
	uid := userID
	resolved, err := s.repo.ResolveUserID(ctx, ptrOrNil(uid))
	if err != nil {
		return Created{}, err
	}
	row.UserID = resolved

	return s.repo.Insert(ctx, row)
}

// ptrOrNil returns nil for "", else &s. Mirrors Node's `x ? ... : null`.
func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// sliceOrNil returns nil for "" (Node `x ? x.slice(0,n) : null`), else the
// value capped to n UTF-16 code units.
func sliceOrNil(s string, n int) *string {
	if s == "" {
		return nil
	}
	v := sliceUTF16(s, n)
	return &v
}

// sliceUTF16 caps s to n UTF-16 code units, mirroring JS String.slice(0, n)
// (which operates on UTF-16 code units). Package-local copy of the errreport
// helper (per CLAUDE.md: keep per-package, do not promote to shared).
func sliceUTF16(s string, n int) string {
	u := utf16.Encode([]rune(s))
	if len(u) <= n {
		return s
	}
	return string(utf16.Decode(u[:n]))
}
