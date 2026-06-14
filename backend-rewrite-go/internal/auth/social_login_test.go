package auth

import (
	"context"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

type fakeSocial struct {
	profile SocialProfile
	err     error
}

func (f fakeSocial) Verify(_ context.Context, _, _ string, _ InboundProfile) (SocialProfile, error) {
	return f.profile, f.err
}

func newSvcSocial(t *testing.T, users UserService, v SocialVerifier) *Service {
	t.Helper()
	return NewService(users, stubSubs{}, stubUsage{}, stubTTL{}, "access-secret", "refresh-secret",
		stubPlans{}, stubSubWriter{}, stubEmail{}, v)
}

func TestSocial_CreateNewUser(t *testing.T) {
	users := newStubUsers()
	svc := newSvcSocial(t, users, fakeSocial{profile: SocialProfile{SocialID: "g1", Email: "g@b.com", Name: "G", EmailVerified: true}})
	res, err := svc.SocialLogin(context.Background(), "google", "tok", InboundProfile{})
	if err != nil {
		t.Fatal(err)
	}
	if res.User.Email != "g@b.com" {
		t.Fatalf("%+v", res.User)
	}
	if !users.created {
		t.Fatal("should create user")
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
}

func TestSocial_UnsupportedProvider(t *testing.T) {
	svc := newSvcSocial(t, newStubUsers(), fakeSocial{})
	_, err := svc.SocialLogin(context.Background(), "myspace", "tok", InboundProfile{})
	assertBadReq(t, err, "Unsupported provider: myspace")
}

func TestSocial_NoEmail(t *testing.T) {
	svc := newSvcSocial(t, newStubUsers(), fakeSocial{profile: SocialProfile{SocialID: "g1", Email: ""}})
	_, err := svc.SocialLogin(context.Background(), "google", "tok", InboundProfile{})
	assertBadReq(t, err, "Email is required for social login")
}

func TestSocial_TakeoverGuard(t *testing.T) {
	users := newStubUsers()
	users.byEmail["g@b.com"] = user.User{ID: "u1", Email: "g@b.com", IsActive: true}
	svc := newSvcSocial(t, users, fakeSocial{profile: SocialProfile{SocialID: "g1", Email: "g@b.com", EmailVerified: false}})
	_, err := svc.SocialLogin(context.Background(), "google", "tok", InboundProfile{})
	assertAuthErr(t, err, "This email is registered. Sign in with your password to link this account.")
}

func TestSocial_LinkVerified(t *testing.T) {
	users := newStubUsers()
	u := user.User{ID: "u1", Email: "g@b.com", Name: "G", IsActive: true}
	users.byEmail["g@b.com"] = u
	users.byID["u1"] = u
	svc := newSvcSocial(t, users, fakeSocial{profile: SocialProfile{SocialID: "g1", Email: "g@b.com", EmailVerified: true}})
	res, err := svc.SocialLogin(context.Background(), "google", "tok", InboundProfile{})
	if err != nil {
		t.Fatal(err)
	}
	if res.User.ID != "u1" {
		t.Fatalf("%+v", res.User)
	}
	if !users.updateCalled {
		t.Fatal("should link")
	}
}

func TestSocial_ExistingBySocialId_Disabled(t *testing.T) {
	users := newStubUsers()
	users.bySocial["google|g1"] = user.User{ID: "u1", Email: "g@b.com", IsActive: false}
	svc := newSvcSocial(t, users, fakeSocial{profile: SocialProfile{SocialID: "g1", Email: "g@b.com", EmailVerified: true}})
	_, err := svc.SocialLogin(context.Background(), "google", "tok", InboundProfile{})
	assertAuthErr(t, err, "Account disabled")
}
