package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// After a reset, the pool serves 5xx until dead conns drain. warmup must retry
// past those and succeed once a request comes back < 500.
func TestWarmup_RecoversAfter5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// first 6 calls simulate dead-conn 57P01 -> 500, then healthy.
		if atomic.AddInt32(&calls, 1) <= 6 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := warmup(srv.Client(), srv.URL, "/plans", 20, time.Millisecond); err != nil {
		t.Fatalf("expected warmup to recover, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 7 {
		t.Fatalf("expected 7 calls (6 failures + 1 success), got %d", got)
	}
}

// A 4xx is a live answer (auth/not-found), not a dead conn — warmup succeeds
// immediately without burning the retry budget.
func TestWarmup_4xxIsSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if err := warmup(srv.Client(), srv.URL, "/plans", 20, time.Millisecond); err != nil {
		t.Fatalf("4xx should count as success, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 call, got %d", got)
	}
}

// If the backend never recovers, warmup must give up and report the last error
// so the gate can FAIL loudly instead of hanging.
func TestWarmup_ExhaustsAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := warmup(srv.Client(), srv.URL, "/plans", 3, time.Millisecond)
	if err == nil {
		t.Fatal("expected warmup to fail after exhausting attempts")
	}
}
