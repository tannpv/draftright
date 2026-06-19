package errreport_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/errreport"
)

// fakeSuggest satisfies the errreport cron's consumer port: it returns a fixed
// set of candidate ids and records every SuggestFix call, failing the ids in
// failOn.
type fakeSuggest struct {
	ids          []string
	candidateErr error
	failOn       map[string]bool

	calledIDs []string
}

func (f *fakeSuggest) AdminFixCandidates(_ context.Context, _ int32) ([]string, error) {
	return f.ids, f.candidateErr
}

func (f *fakeSuggest) SuggestFix(_ context.Context, id string) (errreport.ErrorReportEntity, error) {
	f.calledIDs = append(f.calledIDs, id)
	if f.failOn[id] {
		return errreport.ErrorReportEntity{}, errors.New("boom")
	}
	return errreport.ErrorReportEntity{ID: id}, nil
}

func TestErrorCron_RunsCandidates(t *testing.T) {
	svc := &fakeSuggest{ids: []string{"a", "b"}, failOn: map[string]bool{"b": true}}
	var sleeps int
	cron := errreport.NewCron(svc, func() bool { return false }, func(time.Duration) { sleeps++ })
	res := cron.RunOnce(context.Background())
	require.Equal(t, 1, res.Success)
	require.Equal(t, 1, res.Failed)
	require.Equal(t, []string{"a", "b"}, svc.calledIDs)
	// Node sleeps 1000ms ONLY after a success (inside try, before catch).
	// "a" succeeds → sleep; "b" fails → no sleep.
	require.Equal(t, 1, sleeps)
}

func TestErrorCron_DisabledSkips(t *testing.T) {
	svc := &fakeSuggest{ids: []string{"a"}}
	var sleeps int
	cron := errreport.NewCron(svc, func() bool { return true }, func(time.Duration) { sleeps++ })
	res := cron.RunOnce(context.Background())
	require.Equal(t, 0, res.Success)
	require.Equal(t, 0, res.Failed)
	require.Empty(t, svc.calledIDs)
	require.Equal(t, 0, sleeps)
}

func TestErrorCron_NoCandidates(t *testing.T) {
	svc := &fakeSuggest{ids: nil}
	cron := errreport.NewCron(svc, func() bool { return false }, func(time.Duration) {})
	res := cron.RunOnce(context.Background())
	require.Equal(t, 0, res.Success)
	require.Equal(t, 0, res.Failed)
	require.Empty(t, svc.calledIDs)
}

func TestErrorCron_CandidateErrorReturnsEmpty(t *testing.T) {
	svc := &fakeSuggest{ids: []string{"a"}, candidateErr: errors.New("db down")}
	cron := errreport.NewCron(svc, func() bool { return false }, func(time.Duration) {})
	res := cron.RunOnce(context.Background())
	require.Equal(t, 0, res.Success)
	require.Equal(t, 0, res.Failed)
	require.Empty(t, svc.calledIDs)
}
