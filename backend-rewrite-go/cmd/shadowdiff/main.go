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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// fixture is one replayable request. headers may carry an Authorization
// bearer (a real Node-issued token for authed endpoints).
type fixture struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
	// IgnoreValueOf lists response keys whose value may differ per
	// request (compared for presence only). request_id is the usual one.
	IgnoreValueOf []string `json:"ignore_value_of"`
}

func main() {
	nodeBase := flag.String("node", "", "Node backend base URL")
	goBase := flag.String("go", "", "Go backend base URL")
	dir := flag.String("fixtures", "./fixtures", "fixtures directory")
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

	client := &http.Client{Timeout: 15 * time.Second}
	failed := 0
	for _, f := range fixtures {
		nStatus, nBody, err := send(client, *nodeBase, f)
		if err != nil {
			fmt.Printf("FAIL %s: node request error: %v\n", f.Name, err)
			failed++
			continue
		}
		gStatus, gBody, err := send(client, *goBase, f)
		if err != nil {
			fmt.Printf("FAIL %s: go request error: %v\n", f.Name, err)
			failed++
			continue
		}
		var problems []string
		if nStatus != gStatus {
			problems = append(problems, fmt.Sprintf("status: node=%d go=%d", nStatus, gStatus))
		}
		problems = append(problems, diffJSON(nBody, gBody, f.IgnoreValueOf)...)
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

func loadFixtures(dir string) ([]fixture, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []fixture
	for _, p := range entries {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var f fixture
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if f.Name == "" {
			f.Name = filepath.Base(p)
		}
		out = append(out, f)
	}
	return out, nil
}

func send(c *http.Client, base string, f fixture) (int, []byte, error) {
	var body io.Reader
	if len(f.Body) > 0 {
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
