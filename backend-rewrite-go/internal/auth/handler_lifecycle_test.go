package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

func TestHandler_Register_201(t *testing.T) {
	users := newStubUsers()
	h := NewHandler(newSvc(t, users))
	req := httptest.NewRequest(http.MethodPost, "/auth/register",
		strings.NewReader(`{"email":"a@b.com","password":"password8","name":"Al"}`))
	rr := httptest.NewRecorder()
	h.Register(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["access_token"] == nil || body["access_token"] == "" {
		t.Fatalf("missing access_token: %v", body["access_token"])
	}
	if body["refresh_token"] == nil || body["refresh_token"] == "" {
		t.Fatalf("missing refresh_token: %v", body["refresh_token"])
	}
	u := body["user"].(map[string]any)
	if u["email_verified"] != false {
		t.Fatalf("email_verified: %v", u["email_verified"])
	}
}

func TestHandler_Register_ValidationError_400(t *testing.T) {
	users := newStubUsers()
	h := NewHandler(newSvc(t, users))
	req := httptest.NewRequest(http.MethodPost, "/auth/register",
		strings.NewReader(`{"email":"bad","password":"short","name":""}`))
	rr := httptest.NewRecorder()
	h.Register(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["code"] != "invalid-input" {
		t.Fatalf("code: %v", body["code"])
	}
}

func TestHandler_Register_Duplicate_409(t *testing.T) {
	users := newStubUsers()
	users.byEmail["dup@b.com"] = user.User{ID: "u1", Email: "dup@b.com"}
	h := NewHandler(newSvc(t, users))
	req := httptest.NewRequest(http.MethodPost, "/auth/register",
		strings.NewReader(`{"email":"dup@b.com","password":"password8","name":"Al"}`))
	rr := httptest.NewRecorder()
	h.Register(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["code"] != "conflict" {
		t.Fatalf("code: %v", body["code"])
	}
	if body["error"] != "Email already registered" {
		t.Fatalf("error: %v", body["error"])
	}
}

func TestHandler_VerifyEmail_200(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	users.state["a@b.com"] = authState("123456", &future)
	h := NewHandler(newSvc(t, users))
	req := httptest.NewRequest(http.MethodPost, "/auth/verify-email",
		strings.NewReader(`{"email":"a@b.com","code":"123456"}`))
	rr := httptest.NewRecorder()
	h.VerifyEmail(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["success"] != true {
		t.Fatalf("success: %v", body["success"])
	}
}

func TestHandler_VerifyEmail_Bad_400(t *testing.T) {
	users := newStubUsers()
	h := NewHandler(newSvc(t, users))
	req := httptest.NewRequest(http.MethodPost, "/auth/verify-email",
		strings.NewReader(`{"email":"a@b.com","code":"123456"}`))
	rr := httptest.NewRecorder()
	h.VerifyEmail(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["code"] != "invalid-input" {
		t.Fatalf("code: %v", body["code"])
	}
}

func TestHandler_Resend_200(t *testing.T) {
	users := newStubUsers()
	h := NewHandler(newSvc(t, users))
	req := httptest.NewRequest(http.MethodPost, "/auth/resend-verification",
		strings.NewReader(`{"email":"no@b.com"}`))
	rr := httptest.NewRecorder()
	h.ResendVerification(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["success"] != true {
		t.Fatalf("success: %v", body["success"])
	}
}

func TestHandler_Forgot_200(t *testing.T) {
	users := newStubUsers()
	h := NewHandler(newSvc(t, users))
	req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password",
		strings.NewReader(`{"email":"no@b.com"}`))
	rr := httptest.NewRecorder()
	h.ForgotPassword(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["success"] != true {
		t.Fatalf("success: %v", body["success"])
	}
}

func TestHandler_Reset_ShortPw_400(t *testing.T) {
	users := newStubUsers()
	h := NewHandler(newSvc(t, users))
	req := httptest.NewRequest(http.MethodPost, "/auth/reset-password",
		strings.NewReader(`{"email":"a@b.com","code":"123456","new_password":"short"}`))
	rr := httptest.NewRecorder()
	h.ResetPassword(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["code"] != "invalid-input" {
		t.Fatalf("code: %v", body["code"])
	}
}

func TestHandler_Reset_200(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	st := localState("h")
	st.PasswordResetCode = "123456"
	st.PasswordResetExpires = &future
	users.state["a@b.com"] = st
	h := NewHandler(newSvc(t, users))
	req := httptest.NewRequest(http.MethodPost, "/auth/reset-password",
		strings.NewReader(`{"email":"a@b.com","code":"123456","new_password":"newpassword"}`))
	rr := httptest.NewRecorder()
	h.ResetPassword(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["success"] != true {
		t.Fatalf("success: %v", body["success"])
	}
}
