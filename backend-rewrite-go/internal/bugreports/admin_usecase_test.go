package bugreports

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeAdminRepo records the AdminUpdate / AdminSetFixProposal args and serves a
// fixed row from AdminGet, so the use-case validation + merge + prompt logic is
// testable without a DB.
type fakeAdminRepo struct {
	row      BugReportEntity
	getErr   error
	deleted  bool
	upStatus string
	upNotes  *string
	upTitle  *string
	upTP     *string
	upPublic bool

	fixProposal   string
	fixProposedAt time.Time
	fixStatus     string
}

func (f *fakeAdminRepo) AdminList(context.Context, AdminListFilter) ([]BugReportEntity, int, error) {
	return nil, 0, nil
}
func (f *fakeAdminRepo) AdminGet(context.Context, string) (BugReportEntity, error) {
	if f.getErr != nil {
		return BugReportEntity{}, f.getErr
	}
	return f.row, nil
}
func (f *fakeAdminRepo) AdminDelete(context.Context, string) (bool, error) {
	f.deleted = true
	return true, nil
}
func (f *fakeAdminRepo) AdminUpdate(_ context.Context, _ string, status string, adminNotes, title, targetPlatform *string, isPublic bool) (BugReportEntity, error) {
	f.upStatus, f.upNotes, f.upTitle, f.upTP, f.upPublic = status, adminNotes, title, targetPlatform, isPublic
	out := f.row
	out.Status = status
	return out, nil
}
func (f *fakeAdminRepo) AdminSetFixProposal(_ context.Context, _ string, proposal string, proposedAt time.Time, status string) (BugReportEntity, error) {
	f.fixProposal, f.fixProposedAt, f.fixStatus = proposal, proposedAt, status
	out := f.row
	out.Status = status
	return out, nil
}
func (f *fakeAdminRepo) AdminFixCandidates(context.Context, int32) ([]string, error) {
	return nil, nil
}

type fakeProposer struct {
	system, user string
	out          string
	err          error
}

func (p *fakeProposer) Propose(_ context.Context, system, user string) (string, error) {
	p.system, p.user = system, user
	return p.out, p.err
}

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

func TestUpdate_RejectsBadStatus(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "new"}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	_, err := svc.Update(context.Background(), "id", BugPatch{StatusSet: true, Status: strp("nope")})
	var ve *BugValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want BugValidationError, got %v", err)
	}
	if ve.Msg != "status must be one of: new, reviewing, fix_proposed, resolved, wont_fix" {
		t.Fatalf("bad msg: %q", ve.Msg)
	}
}

// TestUpdate_PresentNullRejectsStatus pins issue #39: an explicit-null status
// (Set=true, value nil) is PRESENT, so Node enters the branch and
// `includes(null)` → false → 400. Go must match, not silently skip.
func TestUpdate_PresentNullRejectsStatus(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "new"}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	_, err := svc.Update(context.Background(), "id", BugPatch{StatusSet: true, Status: nil})
	var ve *BugValidationError
	if !errors.As(err, &ve) || ve.Msg != "status must be one of: new, reviewing, fix_proposed, resolved, wont_fix" {
		t.Fatalf("present-null status should 400, got %v", err)
	}
}

// TestUpdate_PresentNullRejectsTitle: explicit-null title trims to "" → 400.
func TestUpdate_PresentNullRejectsTitle(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "new"}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	_, err := svc.Update(context.Background(), "id", BugPatch{TitleSet: true, Title: nil})
	var ve *BugValidationError
	if !errors.As(err, &ve) || ve.Msg != "title is required (1-80 characters)" {
		t.Fatalf("present-null title should 400, got %v", err)
	}
}

// TestUpdate_PresentNullRejectsTargetPlatform: explicit-null platform → 400.
func TestUpdate_PresentNullRejectsTargetPlatform(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "new"}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	_, err := svc.Update(context.Background(), "id", BugPatch{TargetPlatformSet: true, TargetPlatform: nil})
	var ve *BugValidationError
	if !errors.As(err, &ve) || ve.Msg != "target_platform must be one of: playground, mobile, windows, mac, linux" {
		t.Fatalf("present-null target_platform should 400, got %v", err)
	}
}

// TestUpdate_PresentNullAdminNotesPersistsNull: explicit-null admin_notes is
// present → Node assigns null → SQL NULL. Go passes a nil pointer to the repo.
func TestUpdate_PresentNullAdminNotesPersistsNull(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "new", AdminNotes: strp("old notes")}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	if _, err := svc.Update(context.Background(), "id", BugPatch{AdminNotesSet: true, AdminNotes: nil}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.upNotes != nil {
		t.Fatalf("present-null admin_notes should persist NULL, got %v", *repo.upNotes)
	}
}

// TestUpdate_AbsentAdminNotesKeepsCurrent: an ABSENT admin_notes (Set=false)
// must leave the current value untouched — the distinction issue #39 hinges on.
func TestUpdate_AbsentAdminNotesKeepsCurrent(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "new", AdminNotes: strp("keep me")}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	if _, err := svc.Update(context.Background(), "id", BugPatch{StatusSet: true, Status: strp("reviewing")}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.upNotes == nil || *repo.upNotes != "keep me" {
		t.Fatalf("absent admin_notes should keep current value, got %v", repo.upNotes)
	}
}

func TestUpdate_RejectsBadTitle(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "new"}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	_, err := svc.Update(context.Background(), "id", BugPatch{TitleSet: true, Title: strp("   ")})
	var ve *BugValidationError
	if !errors.As(err, &ve) || ve.Msg != "title is required (1-80 characters)" {
		t.Fatalf("want title validation error, got %v", err)
	}
}

func TestUpdate_RejectsBadTargetPlatform(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "new"}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	_, err := svc.Update(context.Background(), "id", BugPatch{TargetPlatformSet: true, TargetPlatform: strp("toaster")})
	var ve *BugValidationError
	if !errors.As(err, &ve) || ve.Msg != "target_platform must be one of: playground, mobile, windows, mac, linux" {
		t.Fatalf("want target_platform validation error, got %v", err)
	}
}

func TestUpdate_MergesOnlyProvidedFields(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{
		Status:         "new",
		AdminNotes:     strp("old notes"),
		Title:          strp("old title"),
		TargetPlatform: strp("mobile"),
		IsPublic:       false,
	}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	// Only is_public + status provided; notes/title/platform keep current row values.
	_, err := svc.Update(context.Background(), "id", BugPatch{StatusSet: true, Status: strp("reviewing"), IsPublic: boolp(true)})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.upStatus != "reviewing" || !repo.upPublic {
		t.Fatalf("status/public not applied: %q %v", repo.upStatus, repo.upPublic)
	}
	if repo.upNotes == nil || *repo.upNotes != "old notes" {
		t.Fatalf("admin_notes should keep current value, got %v", repo.upNotes)
	}
	if repo.upTitle == nil || *repo.upTitle != "old title" {
		t.Fatalf("title should keep current value, got %v", repo.upTitle)
	}
	if repo.upTP == nil || *repo.upTP != "mobile" {
		t.Fatalf("target_platform should keep current value, got %v", repo.upTP)
	}
}

func TestUpdate_TrimsTitle(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "new"}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	if _, err := svc.Update(context.Background(), "id", BugPatch{TitleSet: true, Title: strp("  Hello  ")}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.upTitle == nil || *repo.upTitle != "Hello" {
		t.Fatalf("title not trimmed: %v", repo.upTitle)
	}
}

func TestSuggestFix_StatusOnlyAdvancesFromNew(t *testing.T) {
	fixed := time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC)
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "reviewing", Description: "x"}}
	prop := &fakeProposer{out: "PROPOSAL"}
	svc := NewAdminService(repo, prop, func() time.Time { return fixed })
	if _, err := svc.SuggestFix(context.Background(), "id"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.fixStatus != "reviewing" {
		t.Fatalf("status must stay reviewing, got %q", repo.fixStatus)
	}
	if repo.fixProposal != "PROPOSAL" || !repo.fixProposedAt.Equal(fixed) {
		t.Fatalf("proposal/time not persisted: %q %v", repo.fixProposal, repo.fixProposedAt)
	}
}

func TestSuggestFix_AdvancesNewToFixProposed(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{Status: "new", Description: "x"}}
	prop := &fakeProposer{out: "P"}
	svc := NewAdminService(repo, prop, time.Now)
	if _, err := svc.SuggestFix(context.Background(), "id"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.fixStatus != "fix_proposed" {
		t.Fatalf("status should become fix_proposed, got %q", repo.fixStatus)
	}
}

func TestSuggestFix_PromptShape(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{
		Source:         "mobile",
		OsInfo:         strp("iOS 17"),
		AppVersion:     nil,
		CreatedAt:      "2026-06-19T10:00:00.000Z",
		UserEmail:      nil,
		ScreenshotPath: strp("/x/y.png"),
		Description:    "it crashes",
		Context:        json.RawMessage(`{"a":1}`),
	}}
	prop := &fakeProposer{out: "P"}
	svc := NewAdminService(repo, prop, time.Now)
	if _, err := svc.SuggestFix(context.Background(), "id"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.HasPrefix(prop.system, "You are a senior software engineer reviewing a user-submitted bug report") {
		t.Fatalf("system prompt mismatch")
	}
	for _, want := range []string{
		"Source: mobile",
		"Platform / OS: iOS 17",
		"App version: unknown",
		"Submitted: 2026-06-19T10:00:00.000Z",
		"User email: (anonymous)",
		"Has screenshot: yes",
		"User's description:\nit crashes",
		"Context: {\n  \"a\": 1\n}",
	} {
		if !strings.Contains(prop.user, want) {
			t.Fatalf("user prompt missing %q in:\n%s", want, prop.user)
		}
	}
}

func TestGetScreenshot_NilWhenNoPath(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	ss, err := svc.GetScreenshot(context.Background(), "id")
	if err != nil || ss != nil {
		t.Fatalf("want nil screenshot, got %v %v", ss, err)
	}
}

func TestGetScreenshot_FilenameFallsBackToBasename(t *testing.T) {
	repo := &fakeAdminRepo{row: BugReportEntity{ScreenshotPath: strp("/var/data/2026/abc.png")}}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	ss, err := svc.GetScreenshot(context.Background(), "id")
	if err != nil || ss == nil {
		t.Fatalf("want screenshot, got %v %v", ss, err)
	}
	if ss.Filename != "abc.png" {
		t.Fatalf("filename fallback wrong: %q", ss.Filename)
	}
}

func TestDelete_404Propagates(t *testing.T) {
	repo := &fakeAdminRepo{getErr: ErrNotFound}
	svc := NewAdminService(repo, &fakeProposer{}, time.Now)
	if err := svc.Delete(context.Background(), "id"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if repo.deleted {
		t.Fatalf("must not delete when row missing")
	}
}
