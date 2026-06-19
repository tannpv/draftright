// Command shadowdiff replays fixture requests against the Node backend
// and the Go backend and reports any status/JSON differences. It is the
// Phase 0 parity gate: every fixture must report PASS before a path is
// flipped in Caddy. No live traffic is tapped — fixtures are authored +
// checked in (no production launch yet, so no real stream exists).
//
// Usage:
//
//	shadowdiff --node=https://api.dev.draftright.info \
//	           --go=http://localhost:3001 --fixtures=./fixtures
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// fixture is one replayable request. headers may carry an Authorization
// bearer (a real Node-issued token for authed endpoints).
type fixture struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
	// RawBody is a verbatim request body sent exactly as written (after
	// {{token}} substitution), used for non-JSON payloads the Body field
	// can't express — chiefly multipart/form-data (POST /bug-reports). Set
	// the boundary via a Content-Type header. When RawBody is non-empty it
	// takes precedence over Body. JSON-string escapes (\r\n, \") decode
	// normally, so a multipart payload can be authored as one JSON string.
	RawBody string `json:"raw_body"`
	// IgnoreValueOf lists response keys whose value may differ per
	// request (compared for presence only). request_id is the usual one.
	IgnoreValueOf []string `json:"ignore_value_of"`
	// StatusOnly compares the HTTP status code only and skips the body diff.
	// For endpoints whose body is non-deterministic by nature and cannot be
	// byte-compared even Node-vs-Node — e.g. /metrics (Prometheus text whose
	// counter/gauge values change every scrape). NOT an escape hatch for a
	// real body mismatch; reserve it for genuinely non-deterministic bodies.
	StatusOnly bool `json:"status_only"`
}

func main() {
	nodeBase := flag.String("node", "", "Node backend base URL")
	goBase := flag.String("go", "", "Go backend base URL")
	dir := flag.String("fixtures", "./fixtures", "fixtures directory")
	maintDSN := flag.String("maint-dsn", "", "maintenance Postgres DSN (postgres db) — enables per-fixture reset")
	template := flag.String("template", "draftright_shadow_tmpl", "frozen template DB name")
	dbNode := flag.String("db-node", "draftright_shadow_node", "Node backend's DB name (reset target)")
	dbGo := flag.String("db-go", "draftright_shadow_go", "Go backend's DB name (reset target)")
	routesFile := flag.String("routes", "", "canonical routes.txt — enables coverage assertion")
	userEmail := flag.String("user-email", "", "bootstrap login email")
	userPass := flag.String("user-pass", "", "bootstrap login password")
	adminEmail := flag.String("admin-email", "", "bootstrap admin email")
	adminPass := flag.String("admin-pass", "", "bootstrap admin password")
	warmupPath := flag.String("warmup-path", "/plans", "cheap DB-hard GET hit after each reset to drain dead pool conns")
	flag.Parse()
	if *nodeBase == "" || *goBase == "" {
		fmt.Fprintln(os.Stderr, "both --node and --go are required")
		os.Exit(2)
	}

	fixtures, err := loadFixtures(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load fixtures: %v\n", err)
		os.Exit(2)
	}

	if *routesFile != "" {
		routes, err := loadRoutes(*routesFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load routes: %v\n", err)
			os.Exit(2)
		}
		if miss := missingRoutes(routes, fixtures); len(miss) > 0 {
			fmt.Fprintf(os.Stderr, "coverage gap — %d routes lack a fixture:\n", len(miss))
			for _, m := range miss {
				fmt.Fprintf(os.Stderr, "   - %s\n", m)
			}
			os.Exit(2)
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}

	vars := map[string]string{}
	if *userEmail != "" {
		var berr error
		vars, berr = bootstrapTokens(client, *nodeBase, *userEmail, *userPass, *adminEmail, *adminPass)
		if berr != nil {
			fmt.Fprintf(os.Stderr, "bootstrap: %v\n", berr)
			os.Exit(2)
		}
	}

	var maint *pgx.Conn
	if *maintDSN != "" {
		var cerr error
		maint, cerr = pgx.Connect(context.Background(), *maintDSN)
		if cerr != nil {
			fmt.Fprintf(os.Stderr, "maint connect: %v\n", cerr)
			os.Exit(2)
		}
		defer maint.Close(context.Background())
	}

	failed := 0
	for _, f := range fixtures {
		if maint != nil {
			if err := resetDB(context.Background(), maint, *dbNode, *template); err != nil {
				fmt.Printf("FAIL %s: reset node db: %v\n", f.Name, err)
				failed++
				continue
			}
			if err := resetDB(context.Background(), maint, *dbGo, *template); err != nil {
				fmt.Printf("FAIL %s: reset go db: %v\n", f.Name, err)
				failed++
				continue
			}
			// The reset terminated both backends' pool connections; warm each
			// until it serves a non-5xx so the fixture lands on a live conn
			// (not a just-killed one that would spuriously 5xx). See warmup.go.
			if err := warmup(client, *nodeBase, *warmupPath, 40, 50*time.Millisecond); err != nil {
				fmt.Printf("FAIL %s: warmup node: %v\n", f.Name, err)
				failed++
				continue
			}
			if err := warmup(client, *goBase, *warmupPath, 40, 50*time.Millisecond); err != nil {
				fmt.Printf("FAIL %s: warmup go: %v\n", f.Name, err)
				failed++
				continue
			}
		}
		ff := substitute(f, vars)
		nStatus, nBody, err := send(client, *nodeBase, ff)
		if err != nil {
			fmt.Printf("FAIL %s: node request error: %v\n", f.Name, err)
			failed++
			continue
		}
		gStatus, gBody, err := send(client, *goBase, ff)
		if err != nil {
			fmt.Printf("FAIL %s: go request error: %v\n", f.Name, err)
			failed++
			continue
		}
		var problems []string
		if nStatus != gStatus {
			problems = append(problems, fmt.Sprintf("status: node=%d go=%d", nStatus, gStatus))
		}
		if !f.StatusOnly {
			problems = append(problems, diffJSON(nBody, gBody, f.IgnoreValueOf)...)
		}
		if len(problems) > 0 {
			failed++
			fmt.Printf("FAIL %s\n", f.Name)
			for _, p := range problems {
				fmt.Printf("   - %s\n", p)
			}
			continue
		}
		fmt.Printf("PASS %s\n", f.Name)
	}

	fmt.Printf("\n%d/%d fixtures passed\n", len(fixtures)-failed, len(fixtures))
	if failed > 0 {
		os.Exit(1)
	}
}

// loadFixtures reads every *.json fixture under dir, recursing into
// module sub-directories. The full gate (run-gate.sh) points at the
// top-level fixtures/ dir while verify-module.sh points at one module
// sub-dir — a recursive walk serves both. WalkDir visits entries in
// lexical order, so loading is deterministic.
func loadFixtures(dir string) ([]fixture, error) {
	var out []fixture
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(p) != ".json" {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var f fixture
		if err := json.Unmarshal(raw, &f); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if f.Name == "" {
			f.Name = filepath.Base(p)
		}
		out = append(out, f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func send(c *http.Client, base string, f fixture) (int, []byte, error) {
	var body io.Reader
	switch {
	case f.RawBody != "":
		body = strings.NewReader(f.RawBody)
	case len(f.Body) > 0:
		body = bytes.NewReader(f.Body)
	}
	req, err := http.NewRequest(f.Method, base+f.Path, body)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range f.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, b, err
}
