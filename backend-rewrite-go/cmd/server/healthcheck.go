package main

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// healthCheckArg is the CLI flag the Docker healthcheck invokes. The final
// image is distroless (no /bin/sh, no wget), so the container cannot run a
// shell-form probe — instead the binary probes itself: `/server -healthcheck`.
// See issue #28.
const healthCheckArg = "-healthcheck"

// healthProbeTimeout bounds the self-probe so a hung server still fails fast
// and the orchestrator can act (restart / mark unhealthy).
const healthProbeTimeout = 3 * time.Second

// healthPort extracts the port to probe from a LISTEN_ADDR value. The server
// may bind ":3001" or "0.0.0.0:3001"; either way the probe targets loopback on
// that port. Falls back to "3001" for a bare/empty/malformed value.
func healthPort(listenAddr string) string {
	if listenAddr == "" {
		return "3001"
	}
	if _, port, err := net.SplitHostPort(listenAddr); err == nil && port != "" {
		return port
	}
	// Not host:port (e.g. ":3001" without host, or just "3001").
	return strings.TrimPrefix(listenAddr, ":")
}

// runHealthProbe GETs url and maps the result to a process exit code:
// 200 → 0 (healthy), anything else (non-200 or transport error) → 1.
func runHealthProbe(url string) int {
	client := &http.Client{Timeout: healthProbeTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return 0
	}
	return 1
}

// healthProbe is the full self-probe: resolve the port from LISTEN_ADDR and
// probe the loopback /health endpoint. Returns the process exit code.
func healthProbe(listenAddr string) int {
	return runHealthProbe("http://127.0.0.1:" + healthPort(listenAddr) + "/health")
}
