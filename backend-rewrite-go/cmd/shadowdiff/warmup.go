package main

import (
	"fmt"
	"net/http"
	"time"
)

// warmupConsecutive is how many back-to-back healthy responses the gate
// requires from each backend after a per-fixture DB reset before firing the
// real fixture. 1 sufficed on the 4GB box; 2 closes the wider drain window on
// the 1vCPU/2GB droplet (issue #66).
const warmupConsecutive = 2

// warmup drains a backend's stale connection pool after a per-fixture DB
// reset. The reset (terminate + DROP + CREATE) kills every open session to the
// target DB; pgxpool (and Node's pg pool) only discover those connections are
// dead when a query runs on them, surfacing as a 5xx (Postgres SQLSTATE 57P01,
// "terminating connection due to administrator command"). The pool destroys the
// dead conn on that failed query and dials a fresh one against the recreated DB,
// so a bounded retry on a cheap DB-hard GET converges on a healthy connection.
//
// The gate sends fixtures serially and pgxpool hands out connections LIFO, so a
// warmed-healthy connection sits on top of the idle stack. On the old 4GB box a
// single success was enough. On the downsized 1vCPU/2GB droplet the reset window
// is wider — the pool can still hold multiple just-killed conns, so ONE success
// doesn't prove the pool is drained; the real fixture occasionally lands on a
// dead conn and spuriously 5xxs, FAILing an otherwise byte-identical fixture
// (issue #66). So warmup now requires `needConsecutive` back-to-back healthy
// responses: any 5xx or transport error resets the streak, forcing the pool to
// hand out healthy conns repeatedly before we trust it.
//
// A response with status < 500 counts as healthy: the connection is alive and
// served the request (a 4xx is a live answer, not a dead conn). Only 5xx or a
// transport error breaks the streak.
func warmup(c *http.Client, base, path string, attempts int, delay time.Duration, needConsecutive int) error {
	if needConsecutive < 1 {
		needConsecutive = 1
	}
	url := base + path
	var lastErr error
	consecutive := 0
	for i := 0; i < attempts; i++ {
		resp, err := c.Get(url)
		if err != nil {
			lastErr = err
			consecutive = 0
		} else {
			status := resp.StatusCode
			resp.Body.Close()
			if status < 500 {
				consecutive++
				if consecutive >= needConsecutive {
					return nil
				}
			} else {
				lastErr = fmt.Errorf("warmup %s: status %d", url, status)
				consecutive = 0
			}
		}
		time.Sleep(delay)
	}
	return fmt.Errorf("warmup %s failed to reach %d consecutive healthy responses after %d attempts: %w",
		url, needConsecutive, attempts, lastErr)
}
