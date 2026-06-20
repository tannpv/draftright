package errreport_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/aiprovider"
	"github.com/tannpv/draftright-rewrite/internal/errreport"
)

// fakeAdminRepo satisfies the errreport adminRepo port for use-case tests.
type fakeAdminRepo struct {
	getEntity errreport.ErrorReportEntity
	getErr    error

	gotStatusText  *string
	gotSetResolved bool
	gotResolvedAt  *time.Time
	gotResolvedBy  *string

	gotProposal       string
	gotProposalStatus int
}

func (f *fakeAdminRepo) AdminList(ctx context.Context, _ errreport.AdminListFilter) ([]errreport.ErrorReportEntity, int, error) {
	return nil, 0, nil
}

func (f *fakeAdminRepo) AdminGet(ctx context.Context, _ string) (errreport.ErrorReportEntity, error) {
	if f.getErr != nil {
		return errreport.ErrorReportEntity{}, f.getErr
	}
	return f.getEntity, nil
}

func (f *fakeAdminRepo) AdminDelete(ctx context.Context, _ string) (bool, error) {
	return false, nil
}

func (f *fakeAdminRepo) AdminSetStatusRaw(ctx context.Context, _ string, statusText *string, setResolved bool, resolvedAt *time.Time, resolvedBy *string) (errreport.ErrorReportEntity, error) {
	f.gotStatusText = statusText
	f.gotSetResolved = setResolved
	f.gotResolvedAt = resolvedAt
	f.gotResolvedBy = resolvedBy
	return f.getEntity, nil
}

func (f *fakeAdminRepo) AdminSetFixProposal(ctx context.Context, _ string, proposal string, status int) (errreport.ErrorReportEntity, error) {
	f.gotProposal = proposal
	f.gotProposalStatus = status
	return f.getEntity, nil
}

func (f *fakeAdminRepo) AdminFixCandidates(ctx context.Context, _ int32) ([]string, error) {
	return nil, nil
}

// fakeProposer satisfies the fixProposer port.
type fakeProposer struct {
	text string
	err  error
}

func (p fakeProposer) Propose(ctx context.Context, system, user string) (string, error) {
	return p.text, p.err
}

func TestAdminSetStatus_ResolvedFields(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, func() time.Time { return now })
	_, err := svc.SetStatusRaw(context.Background(), "e1", json.RawMessage("4"), "")
	require.NoError(t, err)
	require.True(t, repo.gotSetResolved)
	require.Equal(t, "4", *repo.gotStatusText)
	require.Equal(t, &now, repo.gotResolvedAt)
	require.Equal(t, "admin", *repo.gotResolvedBy)
}

func TestAdminSetStatus_ResolvedByPassthrough(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, func() time.Time { return now })
	_, err := svc.SetStatusRaw(context.Background(), "e1", json.RawMessage("5"), "mark")
	require.NoError(t, err)
	require.Equal(t, "5", *repo.gotStatusText)
	require.Equal(t, &now, repo.gotResolvedAt)
	require.Equal(t, "mark", *repo.gotResolvedBy)
}

func TestAdminSetStatus_NonResolvedPreserves(t *testing.T) {
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, _ = svc.SetStatusRaw(context.Background(), "e1", json.RawMessage("2"), "")
	// setResolved=false → the SQL CASE keeps the stored resolved_at/resolved_by;
	// the use case passes nil params (ignored), so Node's preserve-on-reopen holds.
	require.False(t, repo.gotSetResolved)
	require.Equal(t, "2", *repo.gotStatusText)
	require.Nil(t, repo.gotResolvedAt)
	require.Nil(t, repo.gotResolvedBy)
}

// A JSON string "4" must NOT resolve: Node uses strict `body.status === 4`, and
// "4" !== 4. It is still forwarded raw ("4") for Postgres to coerce → 200.
func TestAdminSetStatus_StringFourDoesNotResolve(t *testing.T) {
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, err := svc.SetStatusRaw(context.Background(), "e1", json.RawMessage(`"4"`), "")
	require.NoError(t, err)
	require.False(t, repo.gotSetResolved)
	require.Equal(t, "4", *repo.gotStatusText)
}

// A JSON string "3" forwards the unquoted "3" (Postgres coerces → 200).
func TestAdminSetStatus_StringThreeForwardsUnquoted(t *testing.T) {
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, err := svc.SetStatusRaw(context.Background(), "e1", json.RawMessage(`"3"`), "")
	require.NoError(t, err)
	require.Equal(t, "3", *repo.gotStatusText)
}

// A non-integer number forwards its text ("2.5"); Postgres int4-input rejects it
// (→ 500 at the live layer). Use-case still loads + forwards; never resolves.
func TestAdminSetStatus_FloatForwardsText(t *testing.T) {
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, err := svc.SetStatusRaw(context.Background(), "e1", json.RawMessage("2.5"), "")
	require.NoError(t, err)
	require.False(t, repo.gotSetResolved)
	require.Equal(t, "2.5", *repo.gotStatusText)
}

// A non-numeric string forwards "foo" verbatim for Postgres to reject (→ 500).
func TestAdminSetStatus_NonNumericStringForwardsVerbatim(t *testing.T) {
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, err := svc.SetStatusRaw(context.Background(), "e1", json.RawMessage(`"foo"`), "")
	require.NoError(t, err)
	require.Equal(t, "foo", *repo.gotStatusText)
}

// JSON null forwards a nil text → SQL NULL → not-null violation (→ 500). Never
// resolves; nil pointer, not the literal "null".
func TestAdminSetStatus_NullForwardsNilText(t *testing.T) {
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, err := svc.SetStatusRaw(context.Background(), "e1", json.RawMessage("null"), "")
	require.NoError(t, err)
	require.False(t, repo.gotSetResolved)
	require.Nil(t, repo.gotStatusText)
}

func TestAdminSetStatus_NotFound(t *testing.T) {
	repo := &fakeAdminRepo{getErr: errreport.ErrNotFound}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, err := svc.SetStatusRaw(context.Background(), "missing", json.RawMessage("4"), "")
	require.ErrorIs(t, err, errreport.ErrNotFound)
}

func TestAdminSuggestFix_SetsStatus3(t *testing.T) {
	repo := &fakeAdminRepo{getEntity: errreport.ErrorReportEntity{ID: "e1", Platform: "ios", Count: 3}}
	svc := errreport.NewAdminService(repo, fakeProposer{text: "ROOT CAUSE\n..."}, time.Now)
	_, err := svc.SuggestFix(context.Background(), "e1")
	require.NoError(t, err)
	require.Equal(t, "ROOT CAUSE\n...", repo.gotProposal)
	require.Equal(t, 3, repo.gotProposalStatus)
}

func TestAdminSuggestFix_NotFound(t *testing.T) {
	repo := &fakeAdminRepo{getErr: errreport.ErrNotFound}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, err := svc.SuggestFix(context.Background(), "missing")
	require.ErrorIs(t, err, errreport.ErrNotFound)
}

func TestAdminSuggestFix_NoDefaultProviderPropagates(t *testing.T) {
	repo := &fakeAdminRepo{getEntity: errreport.ErrorReportEntity{ID: "e1", Platform: "ios"}}
	svc := errreport.NewAdminService(repo, fakeProposer{err: aiprovider.ErrNoDefaultProvider}, time.Now)
	_, err := svc.SuggestFix(context.Background(), "e1")
	require.ErrorIs(t, err, aiprovider.ErrNoDefaultProvider)
}

// capturingProposer records the system+user prompts passed to Propose so the
// §3.2 byte-for-byte transcription can be asserted.
type capturingProposer struct {
	system string
	user   string
}

func (p *capturingProposer) Propose(ctx context.Context, system, user string) (string, error) {
	p.system = system
	p.user = user
	return "ok", nil
}

func TestAdminSuggestFix_BuildsPrompt(t *testing.T) {
	appV := "2.4.1"
	etype := "TypeError"
	msg := "x is undefined"
	stack := "frame1\nframe2"
	repo := &fakeAdminRepo{getEntity: errreport.ErrorReportEntity{
		ID:          "e1",
		Platform:    "ios",
		AppVersion:  &appV,
		Severity:    "error",
		Count:       3,
		FirstSeenAt: "2026-06-19T12:00:00.000Z",
		LastSeenAt:  "2026-06-19T12:30:00.000Z",
		ErrorType:   &etype,
		Message:     &msg,
		StackTrace:  &stack,
		Context:     []byte(`{"route":"/x"}`),
	}}
	prop := &capturingProposer{}
	svc := errreport.NewAdminService(repo, prop, time.Now)
	_, err := svc.SuggestFix(context.Background(), "e1")
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(prop.system, "You are a senior software engineer reviewing a production crash report."))
	require.Contains(t, prop.system, "ROOT CAUSE")
	require.Contains(t, prop.system, "CONFIDENCE\n<low | medium | high>")

	require.Equal(t, strings.Join([]string{
		"Platform: ios",
		"App version: 2.4.1",
		"Severity: error",
		"Occurrences: 3",
		"First seen: 2026-06-19T12:00:00.000Z",
		"Last seen: 2026-06-19T12:30:00.000Z",
		"",
		"Error type: TypeError",
		"Message: x is undefined",
		"",
		"Stack trace:",
		"frame1\nframe2",
		"",
		"Context: {\n  \"route\": \"/x\"\n}",
	}, "\n"), prop.user)
}

func TestAdminSuggestFix_PromptDefaults(t *testing.T) {
	repo := &fakeAdminRepo{getEntity: errreport.ErrorReportEntity{
		ID:       "e1",
		Platform: "android",
		Severity: "fatal",
		Count:    1,
	}}
	prop := &capturingProposer{}
	svc := errreport.NewAdminService(repo, prop, time.Now)
	_, err := svc.SuggestFix(context.Background(), "e1")
	require.NoError(t, err)

	lines := strings.Split(prop.user, "\n")
	require.Equal(t, "App version: unknown", lines[1])
	require.Equal(t, "First seen: unknown", lines[4])
	require.Equal(t, "Last seen: unknown", lines[5])
	require.Equal(t, "Error type: (none)", lines[7])
	require.Equal(t, "Message: (none)", lines[8])
	require.Equal(t, "(empty)", lines[11])
	require.Equal(t, "Context: {}", lines[13])
}
