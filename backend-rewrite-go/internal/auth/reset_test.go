package auth

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

func localState(hash string) user.AuthState {
	return user.AuthState{ID: "u1", Email: "a@b.com", Name: "Al",
		PasswordHash: hash, AuthProvider: "local", IsActive: true}
}

func TestForgotPassword_SilentUnknown(t *testing.T) {
	users := newStubUsers()
	svc := newSvc(t, users)
	if err := svc.ForgotPassword(context.Background(), "no@b.com"); err != nil {
		t.Fatal(err)
	}
	if users.updateCalled {
		t.Fatal("no update for unknown")
	}
}

func TestForgotPassword_SilentSocialAccount(t *testing.T) {
	users := newStubUsers()
	st := localState("")
	st.AuthProvider = "google"
	users.state["a@b.com"] = st
	svc := newSvc(t, users)
	if err := svc.ForgotPassword(context.Background(), "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if users.updateCalled {
		t.Fatal("no update for social-only")
	}
}

func TestForgotPassword_SetsResetCode(t *testing.T) {
	h, _ := shared.HashPassword("x")
	users := newStubUsers()
	users.state["a@b.com"] = localState(h)
	svc := newSvc(t, users)
	if err := svc.ForgotPassword(context.Background(), "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if !users.lastPatch.PasswordResetCode.Set || users.lastPatch.PasswordResetCode.Value == nil {
		t.Fatal("should set reset code")
	}
}

func TestResetPassword_ShortPassword(t *testing.T) {
	svc := newSvc(t, newStubUsers())
	assertBadReq(t, svc.ResetPassword(context.Background(), "a@b.com", "123456", "short"), "Password must be at least 8 characters")
}

func TestResetPassword_NoResetCode(t *testing.T) {
	users := newStubUsers()
	users.state["a@b.com"] = localState("h")
	svc := newSvc(t, users)
	assertBadReq(t, svc.ResetPassword(context.Background(), "a@b.com", "123456", "newpassword"), "Invalid or expired reset code")
}

func TestResetPassword_WrongCodeIncrementsAttempts(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	st := localState("h")
	st.PasswordResetCode = "111111"
	st.PasswordResetExpires = &future
	st.PasswordResetAttempts = 1
	users.state["a@b.com"] = st
	svc := newSvc(t, users)
	assertBadReq(t, svc.ResetPassword(context.Background(), "a@b.com", "999999", "newpassword"), "Invalid or expired reset code")
	if users.lastPatch.PasswordResetAttempts == nil || *users.lastPatch.PasswordResetAttempts != 2 {
		t.Fatalf("attempts not incremented: %+v", users.lastPatch.PasswordResetAttempts)
	}
}

func TestResetPassword_BurnsCodeAtCap(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	st := localState("h")
	st.PasswordResetCode = "111111"
	st.PasswordResetExpires = &future
	st.PasswordResetAttempts = 4
	users.state["a@b.com"] = st
	svc := newSvc(t, users)
	assertBadReq(t, svc.ResetPassword(context.Background(), "a@b.com", "999999", "newpassword"), "Invalid or expired reset code")
	if !users.lastPatch.PasswordResetCode.Set || users.lastPatch.PasswordResetCode.Value != nil {
		t.Fatal("should burn code (set null)")
	}
}

func TestResetPassword_Success(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	st := localState("h")
	st.PasswordResetCode = "123456"
	st.PasswordResetExpires = &future
	users.state["a@b.com"] = st
	svc := newSvc(t, users)
	if err := svc.ResetPassword(context.Background(), "a@b.com", "123456", "newpassword"); err != nil {
		t.Fatal(err)
	}
	if users.lastPatch.PasswordHash == nil {
		t.Fatal("should set new hash")
	}
}
