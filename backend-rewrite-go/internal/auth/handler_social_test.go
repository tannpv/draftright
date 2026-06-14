package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_Social_201(t *testing.T) {
	users := newStubUsers()
	h := NewHandler(newSvcSocial(t, users, fakeSocial{profile: SocialProfile{SocialID: "g1", Email: "g@b.com", Name: "G", EmailVerified: true}}))
	req := httptest.NewRequest(http.MethodPost, "/auth/social",
		strings.NewReader(`{"provider":"google","id_token":"tok"}`))
	rr := httptest.NewRecorder()
	h.Social(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rr.Code, rr.Body)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if _, ok := body["access_token"]; !ok {
		t.Fatal("no access_token")
	}
}

func TestHandler_Social_UnsupportedProvider_400(t *testing.T) {
	h := NewHandler(newSvcSocial(t, newStubUsers(), fakeSocial{}))
	req := httptest.NewRequest(http.MethodPost, "/auth/social", strings.NewReader(`{"provider":"x","id_token":"t"}`))
	rr := httptest.NewRecorder()
	h.Social(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rr.Code)
	}
}
