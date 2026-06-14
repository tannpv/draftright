package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyGoogle_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sub":"g1","email":"g@b.com","name":"G","picture":"p","email_verified":"true"}`))
	}))
	defer srv.Close()
	v := &httpVerifier{http: srv.Client(), googleURL: srv.URL + "?id_token="}
	p, err := v.verifyGoogle(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if p.SocialID != "g1" || p.Email != "g@b.com" || !p.EmailVerified {
		t.Fatalf("%+v", p)
	}
}

func TestVerifyGoogle_BadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer srv.Close()
	v := &httpVerifier{http: srv.Client(), googleURL: srv.URL + "?id_token="}
	_, err := v.verifyGoogle(context.Background(), "tok")
	assertAuthErr(t, err, "Invalid Google token")
}

func TestVerifyFacebook_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"f1","name":"F","email":"f@b.com","picture":{"data":{"url":"u"}}}`))
	}))
	defer srv.Close()
	v := &httpVerifier{http: srv.Client(), facebookURL: srv.URL + "?access_token="}
	p, err := v.verifyFacebook(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if p.SocialID != "f1" || p.AvatarURL != "u" || !p.EmailVerified {
		t.Fatalf("%+v", p)
	}
}

func TestVerifyTikTok_TrustsProfile(t *testing.T) {
	v := &httpVerifier{}
	p, err := v.Verify(context.Background(), "tiktok", "openid123", InboundProfile{Email: "t@b.com", Name: "T"})
	if err != nil {
		t.Fatal(err)
	}
	if p.SocialID != "openid123" || p.Email != "t@b.com" || p.EmailVerified {
		t.Fatalf("%+v", p)
	}
}
