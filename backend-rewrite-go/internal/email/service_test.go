package email

import (
	"context"
	"testing"
)

type fakeEQ struct {
	suppressed bool
	logs       []string
	apiKey     string
	from       string
}

func (f *fakeEQ) IsEmailSuppressed(_ context.Context, _ string) (bool, error) {
	return f.suppressed, nil
}
func (f *fakeEQ) InsertEmailLog(_ context.Context, a InsertEmailLogArgs) error {
	f.logs = append(f.logs, a.Status)
	return nil
}
func (f *fakeEQ) GetEmailSettings(_ context.Context) (apiKey, from string, err error) {
	return f.apiKey, f.from, nil
}
func (f *fakeEQ) GetEmailTemplate(_ context.Context, _ string) (subject, html string, ok bool) {
	return "", "", false
}

type fakeSender struct {
	called bool
	id     string
	err    error
	lastTo string
}

func (s *fakeSender) send(_ context.Context, apiKey, from, to, subject, html string) (string, error) {
	s.called = true
	s.lastTo = to
	return s.id, s.err
}

func newServiceWith(q Querier, snd sender, envKey string) *Service {
	return &Service{q: q, cfg: Config{EnvAPIKey: envKey}, client: snd}
}

func TestDeliver_SuppressedSkips(t *testing.T) {
	q := &fakeEQ{suppressed: true}
	snd := &fakeSender{}
	svc := newServiceWith(q, snd, "re_1")
	svc.SendVerification(context.Background(), "x@y.z", "Al", "123456")
	svc.wait()
	if snd.called {
		t.Fatal("must not send to suppressed")
	}
	if len(q.logs) == 0 || q.logs[0] != "suppressed" {
		t.Fatalf("logs: %v", q.logs)
	}
}

func TestDeliver_NoKeySkips(t *testing.T) {
	q := &fakeEQ{}
	snd := &fakeSender{}
	svc := newServiceWith(q, snd, "") // env empty + settings empty
	svc.SendVerification(context.Background(), "x@y.z", "Al", "123456")
	svc.wait()
	if snd.called {
		t.Fatal("no key → no send")
	}
	if q.logs[0] != "skipped" {
		t.Fatalf("logs: %v", q.logs)
	}
}

func TestDeliver_SendsAndLogsSent(t *testing.T) {
	q := &fakeEQ{apiKey: "re_1"}
	snd := &fakeSender{id: "msg_1"}
	svc := newServiceWith(q, snd, "")
	svc.SendVerification(context.Background(), "x@y.z", "Al", "123456")
	svc.wait()
	if !snd.called {
		t.Fatal("should send")
	}
	if q.logs[0] != "sent" {
		t.Fatalf("logs: %v", q.logs)
	}
}

func TestDeliver_SendFailLogsFailed(t *testing.T) {
	q := &fakeEQ{apiKey: "re_1"}
	snd := &fakeSender{err: context.DeadlineExceeded}
	svc := newServiceWith(q, snd, "")
	svc.SendPasswordReset(context.Background(), "x@y.z", "Al", "000111")
	svc.wait()
	if q.logs[0] != "failed" {
		t.Fatalf("logs: %v", q.logs)
	}
}
