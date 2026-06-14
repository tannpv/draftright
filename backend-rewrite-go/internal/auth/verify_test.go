package auth

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

func authState(code string, exp *time.Time) user.AuthState {
	return user.AuthState{ID: "u1", Email: "a@b.com", Name: "Al",
		EmailVerificationCode: code, EmailVerificationExpires: exp}
}

func TestVerifyEmail_Success(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	users.state["a@b.com"] = authState("123456", &future)
	svc := newSvc(t, users)
	if err := svc.VerifyEmail(context.Background(), "A@b.com ", "123456"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if users.lastPatch.EmailVerified == nil || !*users.lastPatch.EmailVerified {
		t.Fatal("should set email_verified")
	}
}

func TestVerifyEmail_NoCodeOnUser(t *testing.T) {
	users := newStubUsers()
	users.state["a@b.com"] = authState("", nil)
	svc := newSvc(t, users)
	assertBadReq(t, svc.VerifyEmail(context.Background(), "a@b.com", "123456"), "Invalid or expired verification code")
}

func TestVerifyEmail_WrongCode(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	users.state["a@b.com"] = authState("111111", &future)
	svc := newSvc(t, users)
	assertBadReq(t, svc.VerifyEmail(context.Background(), "a@b.com", "999999"), "Invalid or expired verification code")
}

func TestVerifyEmail_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	users := newStubUsers()
	users.state["a@b.com"] = authState("123456", &past)
	svc := newSvc(t, users)
	assertBadReq(t, svc.VerifyEmail(context.Background(), "a@b.com", "123456"), "Invalid or expired verification code")
}

func TestVerifyEmail_UnknownUser(t *testing.T) {
	svc := newSvc(t, newStubUsers())
	assertBadReq(t, svc.VerifyEmail(context.Background(), "no@b.com", "123456"), "Invalid or expired verification code")
}

func TestResendVerification_SilentUnknown(t *testing.T) {
	users := newStubUsers()
	svc := newSvc(t, users)
	if err := svc.ResendVerification(context.Background(), "no@b.com"); err != nil {
		t.Fatal(err)
	}
	if users.updateCalled {
		t.Fatal("must not update unknown user")
	}
}

func TestResendVerification_SilentAlreadyVerified(t *testing.T) {
	users := newStubUsers()
	st := authState("", nil)
	st.EmailVerified = true
	users.state["a@b.com"] = st
	svc := newSvc(t, users)
	if err := svc.ResendVerification(context.Background(), "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if users.updateCalled {
		t.Fatal("must not update verified user")
	}
}

func TestResendVerification_SetsNewCode(t *testing.T) {
	users := newStubUsers()
	users.state["a@b.com"] = authState("", nil)
	svc := newSvc(t, users)
	if err := svc.ResendVerification(context.Background(), "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if !users.lastPatch.EmailVerificationCode.Set || users.lastPatch.EmailVerificationCode.Value == nil {
		t.Fatal("should set a new code")
	}
}
