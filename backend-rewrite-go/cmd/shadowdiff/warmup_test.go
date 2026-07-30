package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// After a reset, the pool serves 5xx until dead conns drain. warmup must retry
// past those and succeed once it sees `needConsecutive` healthy responses.
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

	if err := warmup(srv.Client(), srv.URL, "/plans", 20, time.Millisecond, 2); err != nil {
		t.Fatalf("expected warmup to recover, got: %v", err)
	}
	// 6 failures, then 2 consecutive successes (calls 7 and 8).
	if got := atomic.LoadInt32(&calls); got != 8 {
		t.Fatalf("expected 8 calls (6 failures + 2 successes), got %d", got)
	}
}

// A 5xx in the MIDDLE of the streak resets the counter — one healthy blip is
// not enough to trust the pool (the core of #66). Sequence: 200, 500, 200, 200.
func TestWarmup_ResetsStreakOn5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// call 2 is a 5xx that breaks a would-be streak.
		if atomic.AddInt32(&calls, 1) == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := warmup(srv.Client(), srv.URL, "/plans", 20, time.Millisecond, 2); err != nil {
		t.Fatalf("expected warmup to recover, got: %v", err)
	}
	// 1:200(streak=1), 2:500(reset), 3:200(streak=1), 4:200(streak=2 -> done).
	if got := atomic.LoadInt32(&calls); got != 4 {
		t.Fatalf("expected 4 calls (streak reset by mid 5xx), got %d", got)
	}
}

// A 4xx is a live answer (auth/not-found), not a dead conn — it counts toward
// the streak. Two immediate 4xx satisfy needConsecutive=2.
func TestWarmup_4xxIsSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if err := warmup(srv.Client(), srv.URL, "/plans", 20, time.Millisecond, 2); err != nil {
		t.Fatalf("4xx should count as success, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected exactly 2 calls, got %d", got)
	}
}

// If the backend never recovers, warmup must give up and report the last error
// so the gate can FAIL loudly instead of hanging.
func TestWarmup_ExhaustsAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := warmup(srv.Client(), srv.URL, "/plans", 3, time.Millisecond, 2)
	if err == nil {
		t.Fatal("expected warmup to fail after exhausting attempts")
	}
}
