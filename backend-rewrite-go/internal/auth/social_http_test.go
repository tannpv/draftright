package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// googleVerifier returns a verifier whose tokeninfo mock replies with body,
// accepting only aud "aud-ok".
func googleVerifier(t *testing.T, body string) (*httpVerifier, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	v := &httpVerifier{http: srv.Client(), googleURL: srv.URL + "?id_token=", googleAuds: []string{"aud-ok"}}
	return v, srv.Close
}

func TestVerifyGoogle_OK(t *testing.T) {
	v, done := googleVerifier(t, `{"sub":"g1","aud":"aud-ok","iss":"accounts.google.com","email":"g@b.com","name":"G","picture":"p","email_verified":"true"}`)
	defer done()
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
	v := &httpVerifier{http: srv.Client(), googleURL: srv.URL + "?id_token=", googleAuds: []string{"aud-ok"}}
	_, err := v.verifyGoogle(context.Background(), "tok")
	assertAuthErr(t, err, "Invalid Google token")
}

// A token minted for a different OAuth client (aud not in our allow-list) must
// be rejected — this is the account-takeover replay the check exists to stop.
func TestVerifyGoogle_WrongAud_Rejected(t *testing.T) {
	v, done := googleVerifier(t, `{"sub":"g1","aud":"someone-elses-client.apps.googleusercontent.com","iss":"accounts.google.com"}`)
	defer done()
	_, err := v.verifyGoogle(context.Background(), "tok")
	assertAuthErr(t, err, "Invalid Google token")
}

// A token with no aud claim at all must be rejected, not treated as a match.
func TestVerifyGoogle_MissingAud_Rejected(t *testing.T) {
	v, done := googleVerifier(t, `{"sub":"g1","iss":"accounts.google.com"}`)
	defer done()
	_, err := v.verifyGoogle(context.Background(), "tok")
	assertAuthErr(t, err, "Invalid Google token")
}

// An empty allow-list must fail closed, never accept everything.
func TestVerifyGoogle_NoAudiencesConfigured_FailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sub":"g1","aud":"aud-ok","iss":"accounts.google.com"}`))
	}))
	defer srv.Close()
	v := &httpVerifier{http: srv.Client(), googleURL: srv.URL + "?id_token=", googleAuds: nil}
	_, err := v.verifyGoogle(context.Background(), "tok")
	assertAuthErr(t, err, "Google sign-in is not configured")
}

// A token echoing an unexpected issuer must be rejected even with a valid aud.
func TestVerifyGoogle_WrongIssuer_Rejected(t *testing.T) {
	v, done := googleVerifier(t, `{"sub":"g1","aud":"aud-ok","iss":"evil.example.com"}`)
	defer done()
	_, err := v.verifyGoogle(context.Background(), "tok")
	assertAuthErr(t, err, "Invalid Google token")
}

// The https:// issuer spelling is accepted (tokeninfo uses both).
func TestVerifyGoogle_HttpsIssuer_OK(t *testing.T) {
	v, done := googleVerifier(t, `{"sub":"g1","aud":"aud-ok","iss":"https://accounts.google.com","email":"g@b.com"}`)
	defer done()
	if _, err := v.verifyGoogle(context.Background(), "tok"); err != nil {
		t.Fatal(err)
	}
}

// The default allow-list (env unset) accepts the shipped web client id.
func TestNewHTTPSocialVerifier_DefaultGoogleAuds(t *testing.T) {
	v := NewHTTPSocialVerifier("", "")
	if !slices.Contains(v.googleAuds, "22951518033-gf853ftmf4emivffk0su2bik42j7cmai.apps.googleusercontent.com") {
		t.Fatalf("default google auds missing shipped web client id: %v", v.googleAuds)
	}
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
