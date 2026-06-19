package errreport_test

import (
	"context"
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

	gotStatus     int
	gotResolvedAt *time.Time
	gotResolvedBy *string

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

func (f *fakeAdminRepo) AdminSetStatus(ctx context.Context, _ string, status int, resolvedAt *time.Time, resolvedBy *string) (errreport.ErrorReportEntity, error) {
	f.gotStatus = status
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
	_, err := svc.SetStatus(context.Background(), "e1", 4, "")
	require.NoError(t, err)
	require.Equal(t, &now, repo.gotResolvedAt)
	require.Equal(t, "admin", *repo.gotResolvedBy)
}

func TestAdminSetStatus_ResolvedByPassthrough(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, func() time.Time { return now })
	_, err := svc.SetStatus(context.Background(), "e1", 5, "mark")
	require.NoError(t, err)
	require.Equal(t, &now, repo.gotResolvedAt)
	require.Equal(t, "mark", *repo.gotResolvedBy)
}

func TestAdminSetStatus_NonResolvedLeavesNil(t *testing.T) {
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, _ = svc.SetStatus(context.Background(), "e1", 2, "")
	require.Nil(t, repo.gotResolvedAt)
	require.Nil(t, repo.gotResolvedBy)
}

func TestAdminSetStatus_NotFound(t *testing.T) {
	repo := &fakeAdminRepo{getErr: errreport.ErrNotFound}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, err := svc.SetStatus(context.Background(), "missing", 4, "")
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
