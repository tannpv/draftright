package auth

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/plans"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

type stubUsers struct {
	byEmail map[string]user.User
	byID    map[string]user.User
}

func (s stubUsers) ByEmail(_ context.Context, e string) (user.User, error) {
	u, ok := s.byEmail[e]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}
func (s stubUsers) ByID(_ context.Context, id string) (user.User, error) {
	u, ok := s.byID[id]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}
func (s stubUsers) UpdatePasswordHash(context.Context, string, string) error { return nil }
func (s stubUsers) DeleteAccount(context.Context, string) error              { return nil }
func (s stubUsers) Create(context.Context, user.NewUser) (user.User, error)  { return user.User{}, nil }
func (s stubUsers) Update(context.Context, string, user.UserPatch) error     { return nil }
func (s stubUsers) FindBySocialId(context.Context, string, string) (user.User, error) {
	return user.User{}, user.ErrNotFound
}
func (s stubUsers) AuthState(context.Context, string) (user.AuthState, error) {
	return user.AuthState{}, user.ErrNotFound
}

type stubTTL struct{}

func (stubTTL) TokenTTLs(context.Context) (time.Duration, time.Duration, error) {
	return 15 * time.Minute, 90 * 24 * time.Hour, nil
}

func newSvc(t *testing.T, users UserService) *Service {
	t.Helper()
	return NewService(users, stubSubs{}, stubUsage{}, stubTTL{}, "access-secret", "refresh-secret",
		stubPlans{}, stubSubWriter{}, stubEmail{}, stubSocial{})
}

type stubPlans struct{}

func (stubPlans) FindFreePlan(context.Context) (plans.Plan, error) {
	return plans.Plan{ID: "free-plan", Name: "Free", DailyLimit: 10}, nil
}

type stubSubWriter struct{}

func (stubSubWriter) CreateFree(context.Context, string, string) error { return nil }

type stubEmail struct{}

func (stubEmail) SendVerification(context.Context, string, string, string)  {}
func (stubEmail) SendPasswordReset(context.Context, string, string, string) {}

type stubSocial struct{}

func (stubSocial) Verify(context.Context, string, string, InboundProfile) (SocialProfile, error) {
	return SocialProfile{}, nil
}

type stubSubs struct{ sub *subscription.AccountSub }

func (s stubSubs) ActiveByUser(context.Context, string) (*subscription.AccountSub, error) {
	return s.sub, nil
}

type stubUsage struct{ n int }

func (u stubUsage) CountToday(context.Context, string) (int, error) { return u.n, nil }

func TestLogin_HappyPath(t *testing.T) {
	hash, _ := shared.HashPassword("pw123")
	u := user.User{ID: "u1", Email: "a@b.com", Name: "Al", Role: "user", IsActive: true, PasswordHash: hash}
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{"a@b.com": u}})

	res, err := svc.Login(context.Background(), "a@b.com", "pw123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
	if res.User.ID != "u1" || res.User.Email != "a@b.com" || res.User.Name != "Al" {
		t.Fatalf("bad user payload: %+v", res.User)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{}})
	_, err := svc.Login(context.Background(), "no@b.com", "x")
	assertAuthErr(t, err, msgInvalidCredentials)
}

func TestLogin_SocialOnlyAccount(t *testing.T) {
	u := user.User{ID: "u1", Email: "g@b.com", IsActive: true, AuthProvider: "google"} // no PasswordHash
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{"g@b.com": u}})
	_, err := svc.Login(context.Background(), "g@b.com", "x")
	assertAuthErr(t, err, "This account was created with Google. Use the Google button to sign in.")
}

func TestLogin_SocialOnlyTikTok(t *testing.T) {
	u := user.User{ID: "u1", Email: "t@b.com", IsActive: true, AuthProvider: "tiktok"} // no PasswordHash
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{"t@b.com": u}})
	_, err := svc.Login(context.Background(), "t@b.com", "x")
	assertAuthErr(t, err, "This account was created with TikTok. Use the TikTok button to sign in.")
}

func TestLogin_WrongPassword(t *testing.T) {
	hash, _ := shared.HashPassword("right")
	u := user.User{ID: "u1", Email: "a@b.com", IsActive: true, PasswordHash: hash}
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{"a@b.com": u}})
	_, err := svc.Login(context.Background(), "a@b.com", "wrong")
	assertAuthErr(t, err, msgInvalidCredentials)
}

func TestLogin_Disabled(t *testing.T) {
	hash, _ := shared.HashPassword("pw")
	u := user.User{ID: "u1", Email: "a@b.com", IsActive: false, PasswordHash: hash}
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{"a@b.com": u}})
	_, err := svc.Login(context.Background(), "a@b.com", "pw")
	assertAuthErr(t, err, msgAccountDisabled)
}

func assertAuthErr(t *testing.T, err error, wantMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*AuthError)
	if !ok {
		t.Fatalf("want *AuthError, got %T (%v)", err, err)
	}
	if ae.Message != wantMsg {
		t.Fatalf("msg = %q, want %q", ae.Message, wantMsg)
	}
}
