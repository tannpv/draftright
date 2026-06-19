package main

import (
	"fmt"
	"net/http"
	"time"
)

// warmup drains a backend's stale connection pool after a per-fixture DB
// reset. The reset (terminate + DROP + CREATE) kills every open session to the
// target DB; pgxpool (and Node's pg pool) only discover those connections are
// dead when a query runs on them, surfacing as a 5xx (Postgres SQLSTATE 57P01,
// "terminating connection due to administrator command"). The pool destroys the
// dead conn on that failed query and dials a fresh one against the recreated DB,
// so a bounded retry on a cheap DB-hard GET converges on a healthy connection.
//
// The gate sends fixtures serially and pgxpool hands out connections LIFO, so a
// single warmed-healthy connection sits on top of the idle stack and is reused
// for the real fixture — one success is sufficient. Without this, the first
// request after a reset spuriously 5xxs and the fixture FAILs even when both
// backends are byte-identical.
//
// A response with status < 500 counts as success: the connection is alive and
// served the request (a 4xx is a live answer, not a dead conn). Only 5xx or a
// transport error triggers a retry.
func warmup(c *http.Client, base, path string, attempts int, delay time.Duration) error {
	url := base + path
	var lastErr error
	for i := 0; i < attempts; i++ {
		resp, err := c.Get(url)
		if err != nil {
			lastErr = err
		} else {
			status := resp.StatusCode
			resp.Body.Close()
			if status < 500 {
				return nil
			}
			lastErr = fmt.Errorf("warmup %s: status %d", url, status)
		}
		time.Sleep(delay)
	}
	return fmt.Errorf("warmup %s failed after %d attempts: %w", url, attempts, lastErr)
}
