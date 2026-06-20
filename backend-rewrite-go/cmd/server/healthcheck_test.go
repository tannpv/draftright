package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthPort(t *testing.T) {
	cases := map[string]string{
		":3001":          "3001",
		"0.0.0.0:3001":   "3001",
		"127.0.0.1:8080": "8080",
		"3001":           "3001", // bare port, no colon
		"":               "3001", // empty → default
	}
	for in, want := range cases {
		if got := healthPort(in); got != want {
			t.Errorf("healthPort(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRunHealthProbe_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if code := runHealthProbe(srv.URL); code != 0 {
		t.Fatalf("200 health → exit %d, want 0", code)
	}
}

func TestRunHealthProbe_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if code := runHealthProbe(srv.URL); code != 1 {
		t.Fatalf("503 health → exit %d, want 1", code)
	}
}

func TestRunHealthProbe_NoServer(t *testing.T) {
	// Unbound port → transport error → exit 1.
	if code := runHealthProbe("http://127.0.0.1:1/health"); code != 1 {
		t.Fatalf("connection refused → exit %d, want 1", code)
	}
}
