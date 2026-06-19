package bugreports_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/bugreports"
)

// fakeBugSuggest satisfies the bugreports cron's consumer port: it returns a
// fixed set of candidate ids and records every SuggestFix call, failing the ids
// in failOn.
type fakeBugSuggest struct {
	ids          []string
	candidateErr error
	failOn       map[string]bool

	gotLimit  int32
	calledIDs []string
}

func (f *fakeBugSuggest) AdminBugFixCandidates(_ context.Context, limit int32) ([]string, error) {
	f.gotLimit = limit
	return f.ids, f.candidateErr
}

func (f *fakeBugSuggest) SuggestFix(_ context.Context, id string) (bugreports.BugReportEntity, error) {
	f.calledIDs = append(f.calledIDs, id)
	if f.failOn[id] {
		return bugreports.BugReportEntity{}, errors.New("boom")
	}
	return bugreports.BugReportEntity{ID: id}, nil
}

func TestBugCron_RunsCandidates(t *testing.T) {
	svc := &fakeBugSuggest{ids: []string{"a", "b"}, failOn: map[string]bool{"b": true}}
	var sleeps int
	cron := bugreports.NewCron(svc, func() bool { return false }, func(time.Duration) { sleeps++ })
	res := cron.RunOnce(context.Background())
	require.Equal(t, 1, res.Success)
	require.Equal(t, 1, res.Failed)
	require.Equal(t, []string{"a", "b"}, svc.calledIDs)
	// LIMIT 5 per Node BugFixProposalCron.run() (findFixProposalCandidates(5)).
	require.Equal(t, int32(5), svc.gotLimit)
	// Node sleeps 1000ms ONLY after a success (inside try, before catch).
	// "a" succeeds → sleep; "b" fails → no sleep.
	require.Equal(t, 1, sleeps)
}

func TestBugCron_DisabledSkips(t *testing.T) {
	svc := &fakeBugSuggest{ids: []string{"a"}}
	var sleeps int
	cron := bugreports.NewCron(svc, func() bool { return true }, func(time.Duration) { sleeps++ })
	res := cron.RunOnce(context.Background())
	require.Equal(t, 0, res.Success)
	require.Equal(t, 0, res.Failed)
	require.Empty(t, svc.calledIDs)
	require.Equal(t, 0, sleeps)
}

func TestBugCron_NoCandidates(t *testing.T) {
	svc := &fakeBugSuggest{ids: nil}
	cron := bugreports.NewCron(svc, func() bool { return false }, func(time.Duration) {})
	res := cron.RunOnce(context.Background())
	require.Equal(t, 0, res.Success)
	require.Equal(t, 0, res.Failed)
	require.Empty(t, svc.calledIDs)
}

func TestBugCron_CandidateErrorReturnsEmpty(t *testing.T) {
	svc := &fakeBugSuggest{ids: []string{"a"}, candidateErr: errors.New("db down")}
	cron := bugreports.NewCron(svc, func() bool { return false }, func(time.Duration) {})
	res := cron.RunOnce(context.Background())
	require.Equal(t, 0, res.Success)
	require.Equal(t, 0, res.Failed)
	require.Empty(t, svc.calledIDs)
}
