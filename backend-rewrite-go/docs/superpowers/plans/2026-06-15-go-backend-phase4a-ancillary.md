# Go Backend Phase 4a — Ancillary Endpoints Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port three self-contained, LLM-free NestJS ancillary endpoints to Go as byte-identical drop-ins: `GET /ime-packs/manifest` (static catalog), `GET /updates/latest` (desktop update manifest), and `POST /errors` (public crash-report ingest with fingerprint dedup).

**Architecture:** Three independent clean-architecture vertical slices under `internal/`, each composed at `cmd/server/main.go`. `imepacks` is pure in-memory (no DB). `updates` and `errors` add `queries_<feature>.sql` over existing sqlc-introspected tables (`app_releases`, `app_release_policies`, `error_reports`) and a `repo_pg.go`. All three mount in the PUBLIC router block (no JWT gate; `errors` reads an *optional* bearer best-effort). Admin write/read routes for releases and errors are explicitly **out of scope** here — they ship with Phase 4c (admin).

**Tech Stack:** Go 1.26, chi/v5, pgx/v5, sqlc, crypto/sha256, golang-jwt/v5 (existing `auth.Verifier`), shared helpers `shared.WriteJSON` / `shared.WriteError` / `shared.ISOMillis` / `auth.Verifier`.

**Parity authority:** Node source under `/opt/openAi/DraftRight/backend/src/{ime-packs,updates,errors}/`. Where this plan and Node disagree, **NODE WINS** — re-read Node and follow it.

**Standing constraints (every task):**
- Stage ONLY the task's own files. NEVER `git add -A`. `website/src/pages/index.astro` in the parent monorepo MUST stay unstaged.
- TDD: failing test first, then minimal code, then green, then commit.
- `gofmt` clean, `go vet ./...` clean, `go test ./... -race` green before each commit.
- No production/Caddy change. Shadow fixtures are staged for the operator's live gate.
- Error envelope is ALWAYS `{error, code, request_id}` via `shared.WriteError` — never a bare Go error string.
- JSON omitted fields use `omitempty` to match Node's `JSON.stringify` dropping `undefined`.

---

## File Structure

```
internal/imepacks/
  domain.go            # LanguageModule, LanguagePack types
  catalog.go           # the hardcoded 10-entry catalog (Catalog() []LanguageModule)
  handler.go           # GET /ime-packs/manifest
  catalog_test.go      # golden assertions on the 10 entries
  handler_test.go      # response shape + 200

internal/updates/
  domain.go            # Platform consts, AppRelease/Policy value types, compareVersions, maxVersion, buildEnvelope, LatestResponse
  usecase.go           # Service + Repo port (getEffective/listEffective)
  repo_pg.go           # NewPgRepo over sqlc
  handler.go           # GET /updates/latest
  domain_test.go       # version compare + envelope unit tests
  usecase_test.go      # getEffective fallback + listEffective with fakes
  handler_test.go      # full response golden + edge cases

internal/errors/
  domain.go            # CreateErrorReport input, fingerprint, scrub, platform/severity validation, IngestResult
  usecase.go           # Service + Repo port (FindByFingerprint/Insert/BumpDedup)
  repo_pg.go           # NewPgRepo over sqlc
  handler.go           # POST /errors (optional JWT, honeypot, 100KB cap)
  domain_test.go       # fingerprint + scrub + validation unit tests
  usecase_test.go      # new-row + dedup-hit with fakes
  handler_test.go      # honeypot, anonymous, 201 shape, 400 platform, 413

internal/shared/pg/queries_updates.sql   # GetReleasePolicy, GetReleaseByChannel
internal/shared/pg/queries_errors.sql    # FindErrorByFingerprint, InsertErrorReport, BumpErrorReport
internal/shared/router.go                # +3 handler fields, +3 public mounts
cmd/server/main.go                       # wire 3 services/handlers
fixtures/                                # shadow fixtures (Task 18)
```

---

## MODULE A — ime_packs (`GET /ime-packs/manifest`)

Pure in-memory. No DB, no I/O, no auth. Public, HTTP 200. Response: `{"languages":[...10...]}`.

Node authority: `backend/src/ime-packs/ime-packs.service.ts` (the `modules` array, lines 14-66) + `ime-packs.controller.ts` (`{ languages: this.imePacks.catalog() }`).

`PACK_BASE = "https://draftright.info/ime-packs"`.

### Task 1: ime_packs domain types

**Files:**
- Create: `internal/imepacks/domain.go`

- [ ] **Step 1: Write the types**

```go
// Package imepacks serves the static language-pack catalog
// (GET /ime-packs/manifest). It is a byte-identical port of the NestJS
// ime-packs module: a frozen in-memory catalog, no DB, no I/O.
package imepacks

// LanguagePack is a downloadable pack descriptor. Mirrors the Node
// LanguagePack DTO. All fields always present.
type LanguagePack struct {
	URL             string `json:"url"`
	Version         int    `json:"version"`
	SizeBytes       int    `json:"sizeBytes"`
	SHA256          string `json:"sha256"`
	MinEngineVersion int   `json:"minEngineVersion"`
}

// LanguageModule is one catalog entry. pack/wordlistPack are pointers so
// they serialize as omitted (Node drops `undefined`) when nil.
type LanguageModule struct {
	ID           string        `json:"id"`
	DisplayName  string        `json:"displayName"`
	InputMethod  string        `json:"inputMethod"` // composition | candidate | passthrough
	Engine       string        `json:"engine"`      // composition | rime | dictionary | none
	Layout       string        `json:"layout"`      // qwerty | romaji | pinyin
	Bundled      bool          `json:"bundled"`
	Pack         *LanguagePack `json:"pack,omitempty"`
	WordlistPack *LanguagePack `json:"wordlistPack,omitempty"`
}
```

- [ ] **Step 2: Commit** (compiles; tests come with the catalog)

```bash
git add internal/imepacks/domain.go
git commit -m "feat(imepacks-go): LanguageModule/LanguagePack domain types (Phase 4a)"
```

### Task 2: ime_packs catalog (the 10 entries) + golden test

**Files:**
- Create: `internal/imepacks/catalog.go`
- Test: `internal/imepacks/catalog_test.go`

- [ ] **Step 1: Write the failing test** (asserts count, order, and the two real packs byte-for-byte)

```go
package imepacks

import "testing"

func TestCatalog_OrderAndCount(t *testing.T) {
	c := Catalog()
	if len(c) != 10 {
		t.Fatalf("len = %d, want 10", len(c))
	}
	wantIDs := []string{"en", "vi", "fr", "es", "de", "it", "pt", "ko", "ja", "zh"}
	for i, id := range wantIDs {
		if c[i].ID != id {
			t.Errorf("c[%d].ID = %q, want %q", i, c[i].ID, id)
		}
	}
}

func TestCatalog_JaPack(t *testing.T) {
	ja := Catalog()[8]
	if ja.ID != "ja" || ja.Bundled || ja.Layout != "romaji" || ja.Engine != "dictionary" || ja.InputMethod != "candidate" {
		t.Fatalf("ja header wrong: %+v", ja)
	}
	if ja.Pack == nil {
		t.Fatal("ja.Pack nil")
	}
	want := LanguagePack{
		URL:              "https://draftright.info/ime-packs/draftright-ime-ja-v3.pack",
		Version:          3,
		SizeBytes:        2016095,
		SHA256:           "100584d329fa2bbe67d9764ee802b7548a12af9ead01e5e50c599281eaf05282",
		MinEngineVersion: 1,
	}
	if *ja.Pack != want {
		t.Fatalf("ja.Pack = %+v, want %+v", *ja.Pack, want)
	}
	if ja.WordlistPack != nil {
		t.Fatal("ja.WordlistPack should be nil")
	}
}

func TestCatalog_ZhPack(t *testing.T) {
	zh := Catalog()[9]
	if zh.Pack == nil || zh.Layout != "pinyin" {
		t.Fatalf("zh wrong: %+v", zh)
	}
	want := LanguagePack{
		URL:              "https://draftright.info/ime-packs/draftright-ime-zh-v1.pack",
		Version:          1,
		SizeBytes:        1907323,
		SHA256:           "9cc23ff9c85a76e4d38f5991ffd6e0e23e19eceb702d43a39d3e81a562b98b70",
		MinEngineVersion: 1,
	}
	if *zh.Pack != want {
		t.Fatalf("zh.Pack = %+v, want %+v", *zh.Pack, want)
	}
}

func TestCatalog_WordlistPacksOnlyEnViFr(t *testing.T) {
	c := Catalog()
	withWordlist := map[string]bool{"en": true, "vi": true, "fr": true}
	for _, m := range c {
		if withWordlist[m.ID] && m.WordlistPack == nil {
			t.Errorf("%s should have wordlistPack", m.ID)
		}
		if !withWordlist[m.ID] && m.WordlistPack != nil {
			t.Errorf("%s should NOT have wordlistPack", m.ID)
		}
	}
}
```

- [ ] **Step 2: Run, expect FAIL** — `go test ./internal/imepacks/ -run TestCatalog` (Catalog undefined).

- [ ] **Step 3: Write `catalog.go`** — reproduce the Node `modules` array EXACTLY. Re-read `ime-packs.service.ts:14-66` to confirm every field.

```go
package imepacks

const packBase = "https://draftright.info/ime-packs"

func wordlist(stem string) *LanguagePack {
	return &LanguagePack{URL: packBase + "/" + stem, Version: 1, SizeBytes: 0, SHA256: "", MinEngineVersion: 1}
}

// Catalog returns the frozen language-module catalog in declared order.
// Byte-identical to the NestJS ime-packs `modules` array.
func Catalog() []LanguageModule {
	return []LanguageModule{
		{ID: "en", DisplayName: "English", InputMethod: "passthrough", Engine: "none", Layout: "qwerty", Bundled: true,
			WordlistPack: wordlist("draftright-wordlist-en-v1.tsv")},
		{ID: "vi", DisplayName: "Tiếng Việt", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true,
			WordlistPack: wordlist("draftright-wordlist-vi-v1.tsv")},
		{ID: "fr", DisplayName: "Français", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true,
			WordlistPack: wordlist("draftright-wordlist-fr-v1.tsv")},
		{ID: "es", DisplayName: "Español", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true},
		{ID: "de", DisplayName: "Deutsch", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true},
		{ID: "it", DisplayName: "Italiano", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true},
		{ID: "pt", DisplayName: "Português", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true},
		{ID: "ko", DisplayName: "한국어", InputMethod: "composition", Engine: "composition", Layout: "qwerty", Bundled: true},
		{ID: "ja", DisplayName: "日本語", InputMethod: "candidate", Engine: "dictionary", Layout: "romaji", Bundled: false,
			Pack: &LanguagePack{URL: packBase + "/draftright-ime-ja-v3.pack", Version: 3, SizeBytes: 2016095,
				SHA256: "100584d329fa2bbe67d9764ee802b7548a12af9ead01e5e50c599281eaf05282", MinEngineVersion: 1}},
		{ID: "zh", DisplayName: "中文", InputMethod: "candidate", Engine: "dictionary", Layout: "pinyin", Bundled: false,
			Pack: &LanguagePack{URL: packBase + "/draftright-ime-zh-v1.pack", Version: 1, SizeBytes: 1907323,
				SHA256: "9cc23ff9c85a76e4d38f5991ffd6e0e23e19eceb702d43a39d3e81a562b98b70", MinEngineVersion: 1}},
	}
}
```

- [ ] **Step 4: Run, expect PASS** — `go test ./internal/imepacks/ -run TestCatalog`.

- [ ] **Step 5: Commit**

```bash
git add internal/imepacks/catalog.go internal/imepacks/catalog_test.go
git commit -m "feat(imepacks-go): frozen 10-entry catalog + golden tests (Phase 4a)"
```

### Task 3: ime_packs handler + golden JSON test

**Files:**
- Create: `internal/imepacks/handler.go`
- Test: `internal/imepacks/handler_test.go`

- [ ] **Step 1: Write the failing test**

```go
package imepacks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManifest_200AndShape(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()
	h.Manifest(rec, httptest.NewRequest(http.MethodGet, "/ime-packs/manifest", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Languages []LanguageModule `json:"languages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Languages) != 10 {
		t.Fatalf("languages len = %d, want 10", len(body.Languages))
	}
	// es (index 3) is bundled with no packs → neither key present in JSON.
	raw := rec.Body.String()
	esObj := raw[strings.Index(raw, `"id":"es"`):]
	esObj = esObj[:strings.Index(esObj, "}")+1]
	if strings.Contains(esObj, "pack") {
		t.Errorf("es entry must omit pack/wordlistPack, got %s", esObj)
	}
}
```

- [ ] **Step 2: Run, expect FAIL** (Handler undefined).

- [ ] **Step 3: Write `handler.go`**

```go
package imepacks

import (
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler serves GET /ime-packs/manifest. Stateless.
type Handler struct{}

// NewHandler constructs the handler.
func NewHandler() *Handler { return &Handler{} }

// Manifest returns {"languages":[...]} with HTTP 200. Public, no auth.
func (h *Handler) Manifest(w http.ResponseWriter, r *http.Request) {
	shared.WriteJSON(w, http.StatusOK, map[string]any{"languages": Catalog()})
}
```

- [ ] **Step 4: Run, expect PASS**.

- [ ] **Step 5: Commit**

```bash
git add internal/imepacks/handler.go internal/imepacks/handler_test.go
git commit -m "feat(imepacks-go): GET /ime-packs/manifest handler (Phase 4a)"
```

---

## MODULE B — updates (`GET /updates/latest`)

Public read. HTTP 200 always (never 404, even with an empty DB). Two tables. Re-read `backend/src/updates/updates.controller.ts` + `releases.service.ts` before implementing.

Platforms (fixed order): `mac, windows, linux, android, ios`. Channels: `direct, store`.

### Task 4: updates domain — types, version compare, envelope

**Files:**
- Create: `internal/updates/domain.go`
- Test: `internal/updates/domain_test.go`

- [ ] **Step 1: Write the failing tests** (version compare + envelope are the parity-critical bits)

```go
package updates

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int // sign only
	}{
		{"2.2.10", "2.2.9", 1},
		{"2.2.9", "2.2.10", -1},
		{"1.0", "1.0.0", 0},
		{"2.0", "1.9.9", 1},
		{"x.y", "0.0", 0}, // non-numeric → 0
		{"", "", 0},
	}
	for _, c := range cases {
		got := compareVersions(c.a, c.b)
		if sign(got) != c.want {
			t.Errorf("compareVersions(%q,%q) sign = %d, want %d", c.a, c.b, sign(got), c.want)
		}
	}
}

func sign(n int) int {
	if n > 0 {
		return 1
	}
	if n < 0 {
		return -1
	}
	return 0
}

func TestMaxVersion(t *testing.T) {
	if got := maxVersion([]string{"2.2.9", "2.3.0", "1.0.0"}); got != "2.3.0" {
		t.Errorf("maxVersion = %q, want 2.3.0", got)
	}
	if got := maxVersion(nil); got != "" {
		t.Errorf("maxVersion(nil) = %q, want empty", got)
	}
	// "0.0.0" vs seed "" compares to 0 (not >0) → seed wins, stays "".
	if got := maxVersion([]string{"0.0.0"}); got != "" {
		t.Errorf("maxVersion([0.0.0]) = %q, want empty (parity with Node reduce)", got)
	}
}

func TestBuildEnvelope_Anchor(t *testing.T) {
	all := map[string]*Release{
		"mac": {Version: "2.2.9", ReleaseNotes: "mac notes", Required: false},
		"ios": {Version: "2.4.1", ReleaseNotes: "ios notes", Required: true},
	}
	env := buildEnvelope(all, all["ios"])
	if env.Version != "2.4.1" || env.ReleaseNotes != "ios notes" || !env.Required {
		t.Fatalf("anchor envelope = %+v", env)
	}
}

func TestBuildEnvelope_NoAnchorUsesMaxVersionButMacNotes(t *testing.T) {
	all := map[string]*Release{
		"mac": {Version: "2.2.9", ReleaseNotes: "mac notes", Required: true},
		"ios": {Version: "2.4.1", ReleaseNotes: "ios notes", Required: false},
	}
	env := buildEnvelope(all, nil)
	// version = cross-platform max, but notes/required come from mac (legacy asymmetry).
	if env.Version != "2.4.1" {
		t.Errorf("version = %q, want 2.4.1 (max)", env.Version)
	}
	if env.ReleaseNotes != "mac notes" || !env.Required {
		t.Errorf("notes/required must come from mac, got notes=%q required=%v", env.ReleaseNotes, env.Required)
	}
}
```

- [ ] **Step 2: Run, expect FAIL**.

- [ ] **Step 3: Write `domain.go`**

```go
// Package updates serves GET /updates/latest, the desktop/mobile update
// manifest. Byte-identical port of the NestJS updates module's public
// read path. Admin release/policy writes are out of scope (Phase 4c).
package updates

import (
	"strconv"
	"strings"
	"time"
)

// Platforms is the fixed iteration order Node uses everywhere.
var Platforms = []string{"mac", "windows", "linux", "android", "ios"}

// IsPlatform reports whether p is a known platform (validates ?platform=).
func IsPlatform(p string) bool {
	for _, k := range Platforms {
		if k == p {
			return true
		}
	}
	return false
}

// Release is the effective app_releases row for a platform (value type;
// nil means "no enabled release").
type Release struct {
	Platform     string
	Version      string
	DownloadURL  string
	SHA256       string
	ReleaseNotes string
	Required     bool
	Channel      string
	UpdatedAt    time.Time
}

// envelope is the top-level version/notes/required summary.
type envelope struct {
	Version      string
	ReleaseNotes string
	Required     bool
}

// compareVersions ports the Node helper: split on '.', parseInt(seg)||0,
// compare segment-by-segment (missing segment = 0), return a-b sign.
func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		ai, bi := segInt(pa, i), segInt(pb, i)
		if ai != bi {
			return ai - bi
		}
	}
	return 0
}

func segInt(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(parts[i]))
	if err != nil {
		return 0 // parseInt(x,10) || 0
	}
	return v
}

// maxVersion ports the Node reduce: seed "", keep v only when
// compareVersions(v, max) > 0 (strict). Empty input → "".
func maxVersion(versions []string) string {
	max := ""
	for _, v := range versions {
		if v == "" {
			continue
		}
		if compareVersions(v, max) > 0 {
			max = v
		}
	}
	return max
}

// buildEnvelope ports the controller's envelope logic. With an anchor
// (valid ?platform= whose row is non-nil), version/notes/required all come
// from that row. Without an anchor, version = cross-platform max but
// notes/required come from the mac row specifically (legacy asymmetry).
func buildEnvelope(all map[string]*Release, anchor *Release) envelope {
	if anchor != nil {
		return envelope{Version: anchor.Version, ReleaseNotes: anchor.ReleaseNotes, Required: anchor.Required}
	}
	var versions []string
	for _, p := range Platforms {
		if r := all[p]; r != nil {
			versions = append(versions, r.Version)
		}
	}
	top := maxVersion(versions)
	mac := all["mac"]
	env := envelope{Version: top}
	if env.Version == "" && mac != nil {
		env.Version = mac.Version
	}
	if mac != nil {
		env.ReleaseNotes = mac.ReleaseNotes
		env.Required = mac.Required
	}
	return env
}
```

- [ ] **Step 4: Run, expect PASS**.

- [ ] **Step 5: Commit**

```bash
git add internal/updates/domain.go internal/updates/domain_test.go
git commit -m "feat(updates-go): version compare + envelope domain (Phase 4a)"
```

### Task 5: updates sqlc queries + repo

**Files:**
- Create: `internal/shared/pg/queries_updates.sql`
- Create: `internal/updates/repo_pg.go`
- Test: (repo has no unit test; exercised via usecase fakes + the live shadow gate)

- [ ] **Step 1: Write the queries**

```sql
-- internal/shared/pg/queries_updates.sql

-- name: GetReleasePolicy :one
SELECT platform, preferred, store_status, notes, updated_at
FROM app_release_policies
WHERE platform = $1;

-- name: GetEnabledReleaseByChannel :one
SELECT platform, version, download_url, release_notes, required, updated_at, channel, enabled
FROM app_releases
WHERE platform = $1 AND channel = $2 AND enabled = true;
```

NOTE: `app_releases` has no `sha256` column in the sqlc `AppRelease` struct shown by introspection? Re-check: the struct in `models.go` lists `Version, DownloadUrl, ReleaseNotes, Required, UpdatedAt, Channel, Enabled` — **confirm whether `sha256` exists** by running `grep -n sha256 internal/shared/pg/sqlc/models.go`. The Node entity HAS `sha256 varchar(64)`. If the column exists in the DB but is absent from the introspected struct, ADD `sha256` to BOTH SELECTs and regenerate so the struct picks it up. If the column genuinely does not exist in this DB, the response `*_sha256` fields are always `""` — match that. Resolve this BEFORE writing the repo and state which case held.

- [ ] **Step 2: Regenerate** — `sqlc generate`, then `go build ./internal/shared/pg/...`.

- [ ] **Step 3: Write `repo_pg.go`**

```go
package updates

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the updates repo needs.
type Querier interface {
	GetReleasePolicy(ctx context.Context, platform string) (sqlc.AppReleasePolicy, error)
	GetEnabledReleaseByChannel(ctx context.Context, arg sqlc.GetEnabledReleaseByChannelParams) (sqlc.AppRelease, error)
}

// Repo reads effective releases. Ports ReleasesService.getEffective.
type Repo struct{ q Querier }

// NewPgRepo wires the querier.
func NewPgRepo(q Querier) *Repo { return &Repo{q: q} }

// PreferredChannel returns the policy's preferred channel for a platform,
// or "direct" when no policy row exists (Node: policy?.preferred ?? 'direct').
func (r *Repo) PreferredChannel(ctx context.Context, platform string) (string, error) {
	pol, err := r.q.GetReleasePolicy(ctx, platform)
	if errors.Is(err, pgx.ErrNoRows) {
		return "direct", nil
	}
	if err != nil {
		return "", err
	}
	if pol.Preferred == "" {
		return "direct", nil
	}
	return pol.Preferred, nil
}

// EnabledRelease returns the enabled release for (platform, channel), or
// (nil, nil) when none.
func (r *Repo) EnabledRelease(ctx context.Context, platform, channel string) (*Release, error) {
	row, err := r.q.GetEnabledReleaseByChannel(ctx, sqlc.GetEnabledReleaseByChannelParams{Platform: platform, Channel: channel})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rel := &Release{
		Platform:     row.Platform,
		Version:      row.Version,
		DownloadURL:  row.DownloadUrl,
		ReleaseNotes: row.ReleaseNotes,
		Required:     row.Required,
		Channel:      row.Channel,
	}
	if row.UpdatedAt.Valid {
		rel.UpdatedAt = row.UpdatedAt.Time
	}
	// If sha256 exists on the struct after regeneration, set rel.SHA256 = row.Sha256.
	return rel, nil
}
```

(If `sha256` was added to the struct in Step 1, set `rel.SHA256 = row.Sha256` and add it to the SELECTs.)

- [ ] **Step 4: Commit**

```bash
git add internal/shared/pg/queries_updates.sql internal/shared/pg/sqlc internal/updates/repo_pg.go
git commit -m "feat(updates-go): sqlc release/policy queries + pg repo (Phase 4a)"
```

### Task 6: updates use case — getEffective / listEffective

**Files:**
- Create: `internal/updates/usecase.go`
- Test: `internal/updates/usecase_test.go`

- [ ] **Step 1: Write the failing test** (fallback channel + invalid override)

```go
package updates

import (
	"context"
	"testing"
)

type fakeRepo struct {
	preferred map[string]string
	releases  map[string]*Release // key "platform/channel"
}

func (f *fakeRepo) PreferredChannel(_ context.Context, p string) (string, error) {
	if c, ok := f.preferred[p]; ok {
		return c, nil
	}
	return "direct", nil
}
func (f *fakeRepo) EnabledRelease(_ context.Context, p, c string) (*Release, error) {
	return f.releases[p+"/"+c], nil
}

func TestGetEffective_FallbackToOtherChannel(t *testing.T) {
	r := &fakeRepo{
		preferred: map[string]string{"mac": "store"},
		releases:  map[string]*Release{"mac/direct": {Platform: "mac", Version: "2.2.9", Channel: "direct"}},
	}
	s := NewService(r)
	got, _ := s.getEffective(context.Background(), "mac", "")
	if got == nil || got.Channel != "direct" {
		t.Fatalf("expected fallback to direct, got %+v", got)
	}
}

func TestGetEffective_InvalidOverrideIgnored(t *testing.T) {
	r := &fakeRepo{
		preferred: map[string]string{"mac": "direct"},
		releases:  map[string]*Release{"mac/direct": {Platform: "mac", Version: "2.2.9", Channel: "direct"}},
	}
	s := NewService(r)
	got, _ := s.getEffective(context.Background(), "mac", "garbage") // ignored → policy default
	if got == nil || got.Version != "2.2.9" {
		t.Fatalf("garbage override should be ignored, got %+v", got)
	}
}
```

- [ ] **Step 2: Run, expect FAIL**.

- [ ] **Step 3: Write `usecase.go`**

```go
package updates

import "context"

// Repo is the read port (consumer-side interface).
type Repo interface {
	PreferredChannel(ctx context.Context, platform string) (string, error)
	EnabledRelease(ctx context.Context, platform, channel string) (*Release, error)
}

// Service resolves effective releases. Ports ReleasesService read paths.
type Service struct{ repo Repo }

// NewService wires the repo.
func NewService(repo Repo) *Service { return &Service{repo: repo} }

func validChannel(c string) bool { return c == "direct" || c == "store" }

func otherChannel(c string) string {
	if c == "direct" {
		return "store"
	}
	return "direct"
}

// getEffective ports ReleasesService.getEffective: desired channel =
// valid override else policy.preferred; try enabled release there, else
// fall back to the other channel, else nil.
func (s *Service) getEffective(ctx context.Context, platform, override string) (*Release, error) {
	desired := override
	if !validChannel(desired) {
		pref, err := s.repo.PreferredChannel(ctx, platform)
		if err != nil {
			return nil, err
		}
		desired = pref
	}
	rel, err := s.repo.EnabledRelease(ctx, platform, desired)
	if err != nil {
		return nil, err
	}
	if rel != nil {
		return rel, nil
	}
	return s.repo.EnabledRelease(ctx, platform, otherChannel(desired))
}

// listEffective resolves every platform in fixed order.
func (s *Service) listEffective(ctx context.Context, override string) (map[string]*Release, error) {
	out := make(map[string]*Release, len(Platforms))
	for _, p := range Platforms {
		rel, err := s.getEffective(ctx, p, override)
		if err != nil {
			return nil, err
		}
		out[p] = rel
	}
	return out, nil
}
```

- [ ] **Step 4: Run, expect PASS**.

- [ ] **Step 5: Commit**

```bash
git add internal/updates/usecase.go internal/updates/usecase_test.go
git commit -m "feat(updates-go): getEffective/listEffective use case (Phase 4a)"
```

### Task 7: updates handler — GET /updates/latest

**Files:**
- Create: `internal/updates/handler.go`
- Test: `internal/updates/handler_test.go`

- [ ] **Step 1: Write the failing test** (full response golden + empty-DB → 200 with empty `platforms`)

```go
package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubSvc struct{ all map[string]*Release }

func (s stubSvc) listEffective(context.Context, string) (map[string]*Release, error) {
	return s.all, nil
}

func TestLatest_ShapeAndPlatformsOmitNulls(t *testing.T) {
	all := map[string]*Release{
		"mac":     {Platform: "mac", Version: "2.2.9", DownloadURL: "https://x/mac", SHA256: "abc", ReleaseNotes: "n", Required: false, Channel: "direct"},
		"windows": nil, "linux": nil, "android": nil, "ios": nil,
	}
	h := &Handler{svc: stubSvc{all: all}}
	rec := httptest.NewRecorder()
	h.Latest(rec, httptest.NewRequest(http.MethodGet, "/updates/latest", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["mac_url"] != "https://x/mac" || body["windows_url"] != "" {
		t.Errorf("top-level urls wrong: %v", body)
	}
	plats := body["platforms"].(map[string]any)
	if _, ok := plats["mac"]; !ok {
		t.Error("mac must be in platforms")
	}
	if _, ok := plats["windows"]; ok {
		t.Error("null windows must be omitted from platforms")
	}
}

func TestLatest_EmptyDB200(t *testing.T) {
	all := map[string]*Release{"mac": nil, "windows": nil, "linux": nil, "android": nil, "ios": nil}
	h := &Handler{svc: stubSvc{all: all}}
	rec := httptest.NewRecorder()
	h.Latest(rec, httptest.NewRequest(http.MethodGet, "/updates/latest", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["version"] != "" {
		t.Errorf("version = %v, want empty", body["version"])
	}
	if len(body["platforms"].(map[string]any)) != 0 {
		t.Errorf("platforms must be empty")
	}
}
```

- [ ] **Step 2: Run, expect FAIL**.

- [ ] **Step 3: Write `handler.go`** (use an interface so the stub above works; the real Service satisfies it)

```go
package updates

import (
	"context"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// listService is the handler's consumer-side port (Service satisfies it).
type listService interface {
	listEffective(ctx context.Context, override string) (map[string]*Release, error)
}

// Handler serves GET /updates/latest. Public, HTTP 200.
type Handler struct{ svc listService }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// platformEntry is one value in the `platforms` map. Field order + names
// match the Node controller exactly.
type platformEntry struct {
	Version   string `json:"version"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Notes     string `json:"notes"`
	Required  bool   `json:"required"`
	Channel   string `json:"channel"`
	UpdatedAt string `json:"updated_at"`
}

// Latest builds the full update manifest. Never 404s.
func (h *Handler) Latest(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query().Get("channel")
	platform := r.URL.Query().Get("platform")

	all, err := h.svc.listEffective(r.Context(), channel)
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	var anchor *Release
	if IsPlatform(platform) {
		anchor = all[platform]
	}
	env := buildEnvelope(all, anchor)

	urlOf := func(p string) string {
		if r := all[p]; r != nil {
			return r.DownloadURL
		}
		return ""
	}
	shaOf := func(p string) string {
		if r := all[p]; r != nil {
			return r.SHA256
		}
		return ""
	}
	plats := make(map[string]platformEntry)
	for _, p := range Platforms {
		v := all[p]
		if v == nil {
			continue
		}
		plats[p] = platformEntry{
			Version: v.Version, URL: v.DownloadURL, SHA256: v.SHA256,
			Notes: v.ReleaseNotes, Required: v.Required, Channel: v.Channel,
			UpdatedAt: shared.ISOMillis(v.UpdatedAt),
		}
	}

	// Ordered top-level object. Use a struct so field order is fixed.
	resp := struct {
		Version       string                   `json:"version"`
		MacURL        string                   `json:"mac_url"`
		WindowsURL    string                   `json:"windows_url"`
		LinuxURL      string                   `json:"linux_url"`
		AndroidURL    string                   `json:"android_url"`
		IosURL        string                   `json:"ios_url"`
		MacSHA        string                   `json:"mac_sha256"`
		WindowsSHA    string                   `json:"windows_sha256"`
		LinuxSHA      string                   `json:"linux_sha256"`
		AndroidSHA    string                   `json:"android_sha256"`
		IosSHA        string                   `json:"ios_sha256"`
		ReleaseNotes  string                   `json:"release_notes"`
		Required      bool                     `json:"required"`
		Platforms     map[string]platformEntry `json:"platforms"`
	}{
		Version: env.Version,
		MacURL:  urlOf("mac"), WindowsURL: urlOf("windows"), LinuxURL: urlOf("linux"), AndroidURL: urlOf("android"), IosURL: urlOf("ios"),
		MacSHA: shaOf("mac"), WindowsSHA: shaOf("windows"), LinuxSHA: shaOf("linux"), AndroidSHA: shaOf("android"), IosSHA: shaOf("ios"),
		ReleaseNotes: env.ReleaseNotes, Required: env.Required, Platforms: plats,
	}
	shared.WriteJSON(w, http.StatusOK, resp)
}
```

NOTE: `shared.ISOMillis` on a zero `time.Time` yields `"0001-01-01T00:00:00.000Z"`. The shadow fixture for this route MUST list `updated_at` in `ignore_value_of` (it is non-deterministic vs the live DB). The empty-DB test has no platforms entries so this never serializes there.

- [ ] **Step 4: Run, expect PASS**.

- [ ] **Step 5: Commit**

```bash
git add internal/updates/handler.go internal/updates/handler_test.go
git commit -m "feat(updates-go): GET /updates/latest handler (Phase 4a)"
```

---

## MODULE C — errors (`POST /errors`)

Public ingest. Optional best-effort JWT (extract `sub`, ignore failures). Honeypot. sha256 fingerprint dedup (read-then-write). Scrub on new rows. HTTP 201 on success. 100KB body cap → 413.

Re-read `backend/src/errors/errors.controller.ts` + `errors.service.ts` + `dto/create-error-report.dto.ts` before implementing.

### Task 8: errors domain — validation, scrub, fingerprint

**Files:**
- Create: `internal/errors/domain.go`
- Test: `internal/errors/domain_test.go`

NOTE: package name `errors` shadows stdlib `errors` inside this package. Either name the package `errreport` (RECOMMENDED — avoids the shadow and the import-alias churn) or alias stdlib as `stderrors` everywhere. This plan uses **package `errreport`** with directory `internal/errors/`. Confirm the directory/package split with the existing repo convention; if the repo requires package name == dir base, use directory `internal/errreport/`. State which you chose.

```go
// Package errreport ingests client crash reports (POST /errors): optional
// JWT attribution, honeypot drop, sha256 fingerprint dedup, PII scrub.
// Byte-identical port of the NestJS errors module ingest path. Admin
// read/list + the fix-proposal cron are out of scope (later phase).
package errreport

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// AllowedPlatforms / AllowedSeverity mirror the Node service constants.
var AllowedPlatforms = []string{"ios", "android", "macos", "windows", "linux", "web"}
var allowedSeverity = map[string]bool{"fatal": true, "error": true, "warning": true, "info": true}

// CreateErrorReport is the validated ingest input (post-DTO).
type CreateErrorReport struct {
	Platform   string
	AppVersion string
	Severity   string
	ErrorType  string
	Message    string
	StackTrace string
	Context    []byte // raw jsonb, nil when absent
	DeviceID   string
	Website    string // honeypot
}

// PlatformValid reports whether p is in the allowlist.
func PlatformValid(p string) bool {
	for _, k := range AllowedPlatforms {
		if k == p {
			return true
		}
	}
	return false
}

// CoerceSeverity returns s when valid, else "error" (Node default).
func CoerceSeverity(s string) string {
	if allowedSeverity[s] {
		return s
	}
	return "error"
}

// sliceRunes caps s to n UTF-16-ish units. Node uses String.slice (UTF-16
// code units); for the ASCII-heavy stack/message this matches byte slicing
// closely, but to be safe we cap by UTF-16 code units.
func sliceUTF16(s string, n int) string {
	u := utf16Units(s)
	if len(u) <= n {
		return s
	}
	return decodeUTF16(u[:n])
}

var (
	reBearer   = regexp.MustCompile(`Bearer\s+[a-zA-Z0-9._\-]+`)
	rePassword = regexp.MustCompile(`(?i)password["':\s=]+["']?[^"'\s,}]+`)
	reEmail    = regexp.MustCompile(`[\w._%+\-]+@[\w.\-]+\.[a-zA-Z]{2,}`)
)

// Scrub redacts bearer tokens, passwords, and emails. Empty result → "".
func Scrub(s string) string {
	s = reBearer.ReplaceAllString(s, "Bearer [REDACTED]")
	s = rePassword.ReplaceAllString(s, "password=[REDACTED]")
	s = reEmail.ReplaceAllString(s, "[email]")
	return s
}

// Fingerprint = sha256hex( errorType + "::" + first3NonEmptyTrimmedStackLines.join("|") ).
// errorType and stackTrace are the ALREADY-SLICED values (200 / 20000).
func Fingerprint(errorType, stackTrace string) string {
	var frames []string
	for _, ln := range strings.Split(stackTrace, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			frames = append(frames, ln)
		}
		if len(frames) == 3 {
			break
		}
	}
	seed := errorType + "::" + strings.Join(frames, "|")
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}
```

Provide `utf16Units`/`decodeUTF16` via `unicode/utf16` (`utf16.Encode([]rune(s))` → `[]uint16`; decode with `utf16.Decode`). Implement them as small helpers in this file.

- [ ] **Step 1: Write the failing tests**

```go
package errreport

import (
	"strings"
	"testing"
)

func TestScrub(t *testing.T) {
	in := "auth Bearer abc.def-123 failed password=hunter2 for joe@x.com"
	got := Scrub(in)
	if strings.Contains(got, "abc.def") || strings.Contains(got, "hunter2") || strings.Contains(got, "joe@x.com") {
		t.Fatalf("scrub leaked: %q", got)
	}
	if !strings.Contains(got, "Bearer [REDACTED]") || !strings.Contains(got, "[email]") {
		t.Fatalf("scrub markers missing: %q", got)
	}
}

func TestFingerprint_StableOnFirstThreeFrames(t *testing.T) {
	a := Fingerprint("TypeError", "  at f1\n at f2\n at f3\n at f4")
	b := Fingerprint("TypeError", "  at f1\n at f2\n at f3\n at DIFFERENT")
	if a != b {
		t.Fatalf("fingerprint must ignore frames past 3: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("fingerprint len = %d, want 64 hex", len(a))
	}
	c := Fingerprint("RangeError", "  at f1\n at f2\n at f3")
	if a == c {
		t.Fatal("different error_type must change fingerprint")
	}
}

func TestCoerceSeverityAndPlatform(t *testing.T) {
	if CoerceSeverity("nope") != "error" {
		t.Fatal("invalid severity must coerce to error")
	}
	if CoerceSeverity("fatal") != "fatal" {
		t.Fatal("valid severity preserved")
	}
	if PlatformValid("symbian") || !PlatformValid("ios") {
		t.Fatal("platform allowlist wrong")
	}
}
```

- [ ] **Step 2: Run, expect FAIL → implement → Step 3: Run, expect PASS**.

- [ ] **Step 4: Commit**

```bash
git add internal/errors/domain.go internal/errors/domain_test.go
git commit -m "feat(errreport-go): validation + scrub + sha256 fingerprint (Phase 4a)"
```

### Task 9: errors sqlc queries + repo

**Files:**
- Create: `internal/shared/pg/queries_errors.sql`
- Create: `internal/errors/repo_pg.go`

- [ ] **Step 1: Write the queries** (read-then-write dedup; no ON CONFLICT, matching Node)

```sql
-- internal/shared/pg/queries_errors.sql

-- name: FindErrorByFingerprint :one
SELECT * FROM error_reports WHERE fingerprint = $1;

-- name: InsertErrorReport :one
INSERT INTO error_reports
  (platform, app_version, severity, error_type, message, stack_trace, context, user_id, device_id, fingerprint, count, status)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,1,0)
RETURNING id, display_no, count, first_seen_at;

-- name: BumpErrorReport :one
UPDATE error_reports SET
  count = count + 1,
  last_seen_at = now(),
  app_version = COALESCE(sqlc.narg('app_version'), app_version),
  user_id = COALESCE(sqlc.narg('user_id'), user_id),
  device_id = COALESCE(sqlc.narg('device_id'), device_id),
  context = COALESCE(sqlc.narg('context'), context)
WHERE fingerprint = $1
RETURNING id, display_no, count, first_seen_at;
```

NOTE: Node only refreshes a dedup field when the incoming value is provided (truthy). `COALESCE(narg, existing)` replicates that: pass `nil` when the client omitted the field so the existing value is kept. Confirm `error_reports.*` column set matches the sqlc `ErrorReport` struct after generate.

- [ ] **Step 2: Regenerate** — `sqlc generate`; `go build ./internal/shared/pg/...`.

- [ ] **Step 3: Write `repo_pg.go`** — map nullable params via `pgtype`. Provide:
  - `FindByFingerprint(ctx, fp string) (*Existing, error)` returning `(nil,nil)` on `pgx.ErrNoRows`. `Existing` carries `ID string, DisplayNo int64, Count int32, FirstSeenAt time.Time`.
  - `Insert(ctx, NewRow) (*Existing, error)` where `NewRow` carries every column (nullable strings as `*string`, `UserID *string`, `Context []byte`).
  - `BumpDedup(ctx, fp string, appVersion, userID, deviceID *string, context []byte) (*Existing, error)`.

```go
package errreport

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Existing is the dedup/insert projection the handler echoes back.
type Existing struct {
	ID          string
	DisplayNo   int64
	Count       int32
	FirstSeenAt time.Time
}

// NewRow is a fresh insert payload.
type NewRow struct {
	Platform    string
	AppVersion  *string
	Severity    string
	ErrorType   *string
	Message     *string
	StackTrace  *string
	Context     []byte
	UserID      *string
	DeviceID    *string
	Fingerprint string
}

// Querier is the sqlc subset the errors repo needs.
type Querier interface {
	FindErrorByFingerprint(ctx context.Context, fingerprint string) (sqlc.ErrorReport, error)
	InsertErrorReport(ctx context.Context, arg sqlc.InsertErrorReportParams) (sqlc.InsertErrorReportRow, error)
	BumpErrorReport(ctx context.Context, arg sqlc.BumpErrorReportParams) (sqlc.BumpErrorReportRow, error)
}

// Repo is the error_reports adapter.
type Repo struct{ q Querier }

// NewPgRepo wires the querier.
func NewPgRepo(q Querier) *Repo { return &Repo{q: q} }

func toUUID(s *string) pgtype.UUID {
	var u pgtype.UUID
	if s == nil || *s == "" {
		return u
	}
	parsed, err := uuid.Parse(*s)
	if err != nil {
		return u
	}
	u.Bytes = parsed
	u.Valid = true
	return u
}

// FindByFingerprint returns the existing group or (nil,nil).
func (r *Repo) FindByFingerprint(ctx context.Context, fp string) (*Existing, error) {
	row, err := r.q.FindErrorByFingerprint(ctx, fp)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return existingFromRow(row.ID, row.DisplayNo, row.Count, row.FirstSeenAt), nil
}

// Insert creates a new group (count=1, status=0).
func (r *Repo) Insert(ctx context.Context, n NewRow) (*Existing, error) {
	row, err := r.q.InsertErrorReport(ctx, sqlc.InsertErrorReportParams{
		Platform: n.Platform, AppVersion: n.AppVersion, Severity: n.Severity,
		ErrorType: n.ErrorType, Message: n.Message, StackTrace: n.StackTrace,
		Context: n.Context, UserID: toUUID(n.UserID), DeviceID: n.DeviceID, Fingerprint: n.Fingerprint,
	})
	if err != nil {
		return nil, err
	}
	return existingFromRow(row.ID, row.DisplayNo, row.Count, row.FirstSeenAt), nil
}

// BumpDedup increments count + conditionally refreshes fields.
func (r *Repo) BumpDedup(ctx context.Context, fp string, appVersion, userID, deviceID *string, context []byte) (*Existing, error) {
	row, err := r.q.BumpErrorReport(ctx, sqlc.BumpErrorReportParams{
		Fingerprint: fp, AppVersion: appVersion, UserID: toUUID(userID), DeviceID: deviceID, Context: context,
	})
	if err != nil {
		return nil, err
	}
	return existingFromRow(row.ID, row.DisplayNo, row.Count, row.FirstSeenAt), nil
}

func existingFromRow(id pgtype.UUID, displayNo int64, count int32, firstSeen pgtype.Timestamptz) *Existing {
	e := &Existing{DisplayNo: displayNo, Count: count}
	if id.Valid {
		e.ID = uuid.UUID(id.Bytes).String()
	}
	if firstSeen.Valid {
		e.FirstSeenAt = firstSeen.Time
	}
	return e
}
```

NOTE: the generated `BumpErrorReportParams` field types for the `sqlc.narg` columns will be pointer types (`*string`, `pgtype.UUID`, `[]byte`). Verify after `sqlc generate` and adjust the struct literal field names/types to match exactly.

- [ ] **Step 4: Commit**

```bash
git add internal/shared/pg/queries_errors.sql internal/shared/pg/sqlc internal/errors/repo_pg.go
git commit -m "feat(errreport-go): sqlc dedup queries + pg repo (Phase 4a)"
```

### Task 10: errors use case — ingest (new + dedup)

**Files:**
- Create: `internal/errors/usecase.go`
- Test: `internal/errors/usecase_test.go`

- [ ] **Step 1: Write the failing test** (new row scrubs; dedup hit does not insert)

```go
package errreport

import (
	"context"
	"testing"
)

type fakeRepo struct {
	existing  *Existing
	inserted  *NewRow
	bumped    bool
}

func (f *fakeRepo) FindByFingerprint(context.Context, string) (*Existing, error) { return f.existing, nil }
func (f *fakeRepo) Insert(_ context.Context, n NewRow) (*Existing, error) {
	f.inserted = &n
	return &Existing{ID: "new-id", DisplayNo: 7, Count: 1}, nil
}
func (f *fakeRepo) BumpDedup(context.Context, string, *string, *string, *string, []byte) (*Existing, error) {
	f.bumped = true
	return &Existing{ID: "old-id", DisplayNo: 3, Count: 5}, nil
}

func TestIngest_NewRowScrubsAndInserts(t *testing.T) {
	f := &fakeRepo{existing: nil}
	s := NewService(f)
	res, err := s.Ingest(context.Background(), CreateErrorReport{
		Platform: "ios", Message: "token Bearer abc.def leaked", StackTrace: "at f1\nat f2",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if f.inserted == nil {
		t.Fatal("expected insert")
	}
	if f.inserted.Message == nil || *f.inserted.Message == "token Bearer abc.def leaked" {
		t.Fatalf("message not scrubbed: %v", f.inserted.Message)
	}
	if res.Count != 1 {
		t.Fatalf("count = %d, want 1", res.Count)
	}
}

func TestIngest_DedupHitBumps(t *testing.T) {
	f := &fakeRepo{existing: &Existing{ID: "old-id", DisplayNo: 3, Count: 4}}
	s := NewService(f)
	res, _ := s.Ingest(context.Background(), CreateErrorReport{Platform: "ios", StackTrace: "at f1"}, "")
	if f.inserted != nil {
		t.Fatal("dedup hit must NOT insert")
	}
	if !f.bumped || res.Count != 5 {
		t.Fatalf("expected bump, count=%d", res.Count)
	}
}

func TestIngest_InvalidPlatform400(t *testing.T) {
	s := NewService(&fakeRepo{})
	_, err := s.Ingest(context.Background(), CreateErrorReport{Platform: "symbian"}, "")
	if err == nil {
		t.Fatal("expected error for bad platform")
	}
}
```

- [ ] **Step 2: Run, expect FAIL**.

- [ ] **Step 3: Write `usecase.go`**

```go
package errreport

import (
	"context"
	"errors"
)

// ErrInvalidPlatform is returned (→ 400) for a platform outside the allowlist.
var ErrInvalidPlatform = errors.New("platform must be one of: ios, android, macos, windows, linux, web")

// Repo is the consumer-side port.
type Repo interface {
	FindByFingerprint(ctx context.Context, fp string) (*Existing, error)
	Insert(ctx context.Context, n NewRow) (*Existing, error)
	BumpDedup(ctx context.Context, fp string, appVersion, userID, deviceID *string, context []byte) (*Existing, error)
}

// Service ingests error reports.
type Service struct{ repo Repo }

// NewService wires the repo.
func NewService(repo Repo) *Service { return &Service{repo: repo} }

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Ingest ports PaymentService... no — ports ErrorsService.ingest:
// validate platform, coerce severity, slice, fingerprint, dedup-or-insert.
// userID is the best-effort JWT subject ("" when anonymous).
func (s *Service) Ingest(ctx context.Context, in CreateErrorReport, userID string) (*Existing, error) {
	if !PlatformValid(in.Platform) {
		return nil, ErrInvalidPlatform
	}
	severity := CoerceSeverity(in.Severity)
	message := sliceUTF16(in.Message, 5000)
	stack := sliceUTF16(in.StackTrace, 20000)
	errType := sliceUTF16(in.ErrorType, 200)
	fp := Fingerprint(errType, stack)

	existing, err := s.repo.FindByFingerprint(ctx, fp)
	if err != nil {
		return nil, err
	}
	var uid *string
	if userID != "" {
		uid = &userID
	}
	devID := ptrOrNil(sliceUTF16(in.DeviceID, 100))
	var ctxBytes []byte
	if len(in.Context) > 0 {
		ctxBytes = in.Context
	}

	if existing != nil {
		return s.repo.BumpDedup(ctx, fp, ptrOrNil(in.AppVersion), uid, devID, ctxBytes)
	}
	// New row: scrub message + stack (→ nil when empty after scrub).
	scrubbedMsg := Scrub(message)
	scrubbedStack := Scrub(stack)
	return s.repo.Insert(ctx, NewRow{
		Platform:    in.Platform,
		AppVersion:  ptrOrNil(in.AppVersion),
		Severity:    severity,
		ErrorType:   ptrOrNil(errType),
		Message:     ptrOrNil(scrubbedMsg),
		StackTrace:  ptrOrNil(scrubbedStack),
		Context:     ctxBytes,
		UserID:      uid,
		DeviceID:    devID,
		Fingerprint: fp,
	})
}
```

Fix the stray comment line ("ports PaymentService... no —") before committing — it must read `// Ingest ports ErrorsService.ingest: ...`.

- [ ] **Step 4: Run, expect PASS**.

- [ ] **Step 5: Commit**

```bash
git add internal/errors/usecase.go internal/errors/usecase_test.go
git commit -m "feat(errreport-go): ingest use case (new + dedup) (Phase 4a)"
```

### Task 11: errors handler — POST /errors (optional JWT, honeypot, 413)

**Files:**
- Create: `internal/errors/handler.go`
- Test: `internal/errors/handler_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package errreport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubIngest struct {
	res *Existing
	err error
	got CreateErrorReport
	uid string
}

func (s *stubIngest) Ingest(_ context.Context, in CreateErrorReport, uid string) (*Existing, error) {
	s.got, s.uid = in, uid
	return s.res, s.err
}

func newHandler(s ingestService) *Handler { return &Handler{svc: s, verifier: nil} }

func TestErrors_HoneypotDropsWithoutRef(t *testing.T) {
	st := &stubIngest{}
	h := newHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(`{"platform":"ios","website":"spam"}`))
	h.Ingest(rec, req)
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["ok"] != true || body["id"] != nil || body["count"] != float64(0) {
		t.Fatalf("honeypot body wrong: %v", body)
	}
	if _, hasRef := body["ref"]; hasRef {
		t.Error("honeypot response must NOT contain ref")
	}
	if st.got.Platform != "" {
		t.Error("honeypot must not reach the service")
	}
}

func TestErrors_NormalIngest201WithRef(t *testing.T) {
	st := &stubIngest{res: &Existing{ID: "abc", DisplayNo: 42, Count: 1}}
	h := newHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(`{"platform":"ios","message":"boom"}`))
	h.Ingest(rec, req)
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["ref"] != "ERR-42" || body["id"] != "abc" || body["count"] != float64(1) {
		t.Fatalf("body = %v", body)
	}
}

func TestErrors_BadPlatform400(t *testing.T) {
	st := &stubIngest{err: ErrInvalidPlatform}
	h := newHandler(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(`{"platform":"symbian"}`))
	h.Ingest(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", body["code"])
	}
}
```

- [ ] **Step 2: Run, expect FAIL**.

- [ ] **Step 3: Write `handler.go`**

```go
package errreport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

const maxBodyBytes = 100 * 1024 // Express default json limit → 413 past this

// ingestService is the handler's consumer-side port (Service satisfies it).
type ingestService interface {
	Ingest(ctx context.Context, in CreateErrorReport, userID string) (*Existing, error)
}

// Handler serves POST /errors. Public; reads an OPTIONAL bearer for
// attribution.
type Handler struct {
	svc      ingestService
	verifier *auth.Verifier // may be nil in tests
}

// NewHandler wires the service + the access-token verifier.
func NewHandler(svc *Service, v *auth.Verifier) *Handler { return &Handler{svc: svc, verifier: v} }

// requestBody mirrors the Node DTO field names (snake_case JSON).
type requestBody struct {
	Platform   string          `json:"platform"`
	AppVersion string          `json:"app_version"`
	Severity   string          `json:"severity"`
	ErrorType  string          `json:"error_type"`
	Message    string          `json:"message"`
	StackTrace string          `json:"stack_trace"`
	Context    json.RawMessage `json:"context"`
	DeviceID   string          `json:"device_id"`
	Website    string          `json:"website"` // honeypot
}

// Ingest handles POST /errors → 201.
func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body requestBody
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		// MaxBytesReader trips a "http: request body too large" error → 413.
		if strings.Contains(err.Error(), "too large") {
			shared.WriteError(w, r, "http-413", "request entity too large")
			return
		}
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}

	// Honeypot: any non-empty `website` → silent 201 drop, distinct body (no ref).
	if strings.TrimSpace(body.Website) != "" {
		shared.WriteJSON(w, http.StatusCreated, map[string]any{
			"ok": true, "id": nil, "fingerprint": nil, "count": 0, "first_seen_at": nil,
		})
		return
	}

	userID := h.optionalUserID(r)

	var ctxBytes []byte
	if len(body.Context) > 0 && string(body.Context) != "null" {
		ctxBytes = []byte(body.Context)
	}
	res, err := h.svc.Ingest(r.Context(), CreateErrorReport{
		Platform: body.Platform, AppVersion: body.AppVersion, Severity: body.Severity,
		ErrorType: body.ErrorType, Message: body.Message, StackTrace: body.StackTrace,
		Context: ctxBytes, DeviceID: body.DeviceID, Website: body.Website,
	}, userID)
	if err != nil {
		if err == ErrInvalidPlatform {
			shared.WriteError(w, r, "invalid-input", err.Error())
			return
		}
		shared.WriteError(w, r, "internal", err.Error())
		return
	}

	shared.WriteJSON(w, http.StatusCreated, map[string]any{
		"ok":            true,
		"id":            res.ID,
		"ref":           "ERR-" + strconv.FormatInt(res.DisplayNo, 10),
		"fingerprint":   nil, // set below
		"count":         res.Count,
		"first_seen_at": shared.ISOMillis(res.FirstSeenAt),
	})
}

// optionalUserID extracts `sub` from a best-effort bearer; "" on any failure.
func (h *Handler) optionalUserID(r *http.Request) string {
	if h.verifier == nil {
		return ""
	}
	authz := r.Header.Get("Authorization")
	const p = "Bearer "
	if len(authz) <= len(p) || !strings.EqualFold(authz[:len(p)], p) {
		return ""
	}
	claims, err := h.verifier.Verify(strings.TrimSpace(authz[len(p):]))
	if err != nil {
		return ""
	}
	return claims.UserID()
}
```

PARITY FIX REQUIRED in Step 3: the normal-success body must include the real `fingerprint` (sha256 hex), not `nil`. The fingerprint is computed in the use case but not currently returned. Extend `Existing` with a `Fingerprint string` field, set it in `Insert`/`BumpDedup`/`FindByFingerprint` (the repo already knows `fp`), and emit `res.Fingerprint` here. Update the use case to stamp `existing.Fingerprint = fp` before returning in BOTH branches. Re-run tests. (The handler test above doesn't assert fingerprint; add an assertion `body["fingerprint"]` is a 64-char string in the normal-ingest test.)

- [ ] **Step 4: Run, expect PASS** after the fingerprint wiring.

- [ ] **Step 5: Commit**

```bash
git add internal/errors/handler.go internal/errors/handler_test.go internal/errors/usecase.go internal/errors/repo_pg.go
git commit -m "feat(errreport-go): POST /errors handler (optional JWT, honeypot, 413) (Phase 4a)"
```

---

## Task 12: Router fields + public mounts (all 3 modules)

**Files:**
- Modify: `internal/shared/router.go`

- [ ] **Step 1: Add 3 handler fields** to the `Router` struct (beside the payment webhook fields):

```go
	ImePacksManifest http.Handler
	UpdatesLatest    http.Handler
	ErrorsIngest     http.Handler
```

- [ ] **Step 2: Mount in the PUBLIC block** (before the api/auth group), nil-guarded like the webhook mounts:

```go
	if r.ImePacksManifest != nil {
		mux.Method(http.MethodGet, "/ime-packs/manifest", r.ImePacksManifest)
	}
	if r.UpdatesLatest != nil {
		mux.Method(http.MethodGet, "/updates/latest", r.UpdatesLatest)
	}
	if r.ErrorsIngest != nil {
		mux.Method(http.MethodPost, "/errors", r.ErrorsIngest)
	}
```

- [ ] **Step 3: Add a router test** (`internal/shared/router_*_test.go`) asserting all 3 routes are reachable WITHOUT a JWT (status != 401). Mount trivial stub handlers. Run, expect PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/shared/router.go internal/shared/router_phase4a_test.go
git commit -m "feat(router-go): mount ime-packs/updates/errors public routes (Phase 4a)"
```

## Task 13: main.go wiring

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Build the 3 verticals** and assign router fields. Reuse the existing pool `q` (sqlc Queries), the existing `auth.Verifier` for the access secret (the same one `RequireAuth` uses), and `imepacks.NewHandler()`:

```go
	// Phase 4a ancillary endpoints
	imeHandler := imepacks.NewHandler()
	updatesSvc := updates.NewService(updates.NewPgRepo(q))
	updatesHandler := updates.NewHandler(updatesSvc)
	errSvc := errreport.NewService(errreport.NewPgRepo(q))
	errHandler := errreport.NewHandler(errSvc, accessVerifier) // same verifier as RequireAuth
```

Then in the `&shared.Router{...}` literal:

```go
		ImePacksManifest: http.HandlerFunc(imeHandler.Manifest),
		UpdatesLatest:    http.HandlerFunc(updatesHandler.Latest),
		ErrorsIngest:     http.HandlerFunc(errHandler.Ingest),
```

Confirm the variable name of the existing access-token verifier in `main.go` (grep `NewVerifier`) and reuse it — do NOT construct a second one.

- [ ] **Step 2: Build + smoke** — `go build ./...`; `go test ./... -race`. Expect green.

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server-go): wire ime-packs/updates/errors handlers (Phase 4a)"
```

## Task 14: Full-suite verification gate

- [ ] **Step 1:** `go test ./... -race` — all green.
- [ ] **Step 2:** `gofmt -l .` — prints nothing (fix any listed file).
- [ ] **Step 3:** `go vet ./...` — clean.
- [ ] **Step 4:** Confirm the 3 new routes are public: re-read `router.go` and verify the mounts sit BEFORE the api group. 
- [ ] **Step 5:** No commit needed if clean (gofmt/vet produced no changes).

## Task 15: Shadow fixtures

**Files:**
- Create: `fixtures/imepacks_manifest.json`
- Create: `fixtures/updates_latest.json`
- Create: `fixtures/errors_ingest_honeypot.json`
- Create: `fixtures/errors_ingest_bad_platform.json`

Mirror the existing fixture schema (`name, method, path, headers, body?, ignore_value_of, _note`). Re-read `fixtures/payment_status.json` for the exact shape.

- [ ] **`imepacks_manifest.json`** — `GET /ime-packs/manifest`, no headers, no body. Fully deterministic (static catalog) → `ignore_value_of: []`. Both backends return 200 + the identical 10-entry `languages` array. `_note`: pure in-memory catalog, byte-deterministic, nothing to ignore.

- [ ] **`updates_latest.json`** — `GET /updates/latest`, no headers, no body. `ignore_value_of: ["updated_at"]` (per-platform `updated_at` reflects live DB write time). `_note`: response shape is deterministic given the seeded `app_releases`/`app_release_policies`; only the nested `platforms.<p>.updated_at` is non-deterministic. PREREQUISITE: the shadow DB must hold identical release rows for Node and Go (same Postgres → automatic). If the DB has no release rows, both return the empty-manifest 200 (`platforms:{}`) — also identical.

- [ ] **`errors_ingest_honeypot.json`** — `POST /errors`, body `{"platform":"ios","website":"x"}`, header `Content-Type: application/json`. Both backends return **201** `{ok:true, id:null, fingerprint:null, count:0, first_seen_at:null}` — fully deterministic, writes nothing. `ignore_value_of: []`. `_note`: honeypot drop path; safest errors parity check (no DB write, no non-determinism).

- [ ] **`errors_ingest_bad_platform.json`** — `POST /errors`, body `{"platform":"symbian"}`. Both return **400** `{error:"platform must be one of: ios, android, macos, windows, linux, web", code:"invalid-input", request_id:<uuid>}`. `ignore_value_of: ["request_id"]`. `_note`: deterministic validation rejection; PREREQUISITE: none (validation runs before any DB/secret access).

DO NOT author a fixture for the normal (DB-writing) `POST /errors` success path — it mutates state and returns a non-deterministic `id`/`first_seen_at`/`display_no`, so it can't be a clean shadow check. Note this exclusion in the commit message.

- [ ] **Commit**

```bash
git add fixtures/imepacks_manifest.json fixtures/updates_latest.json fixtures/errors_ingest_honeypot.json fixtures/errors_ingest_bad_platform.json
git commit -m "test(phase4a-go): shadow fixtures for ime-packs/updates/errors (Phase 4a)"
```

---

## Self-Review (parity coverage)

| Node behavior | Task |
|---|---|
| `GET /ime-packs/manifest` public 200, `{languages:[10]}`, pack/wordlistPack omitted when absent | 1-3 |
| ja/zh real pack sha256 + sizeBytes byte-for-byte | 2 |
| `GET /updates/latest` public 200, never 404 | 4-7 |
| Top-level `*_url`/`*_sha256` always present (""), `platforms` omits null rows | 7 |
| `getEffective` channel fallback + `enabled=true` filter | 5,6 |
| Envelope: anchor branch vs max-version branch; notes/required from mac in no-anchor branch | 4 |
| `compareVersions` parseInt||0 + reduce seed "" semantics | 4 |
| `platforms.<p>` uses key `notes` (not release_notes), ISO `updated_at` | 7 |
| `POST /errors` public 201, optional best-effort JWT `sub` | 8-11 |
| Honeypot `website` → 201 `{ok,id:null,...}` WITHOUT `ref` | 11 |
| Platform allowlist → 400 exact message; severity coerce to "error" | 8,10,11 |
| sha256 fingerprint of `errorType::first3frames`; dedup read-then-write | 8,9,10 |
| Scrub (bearer/password/email) on NEW rows only; empty→null | 8,10 |
| `message`/`stack`/`error_type` slice 5000/20000/200; device_id 100 | 8,10 |
| Dedup hit bumps count + conditionally refreshes fields (COALESCE) | 9,10 |
| `ref = "ERR-"+display_no`; `first_seen_at` ISO | 11 |
| 100KB body → 413 `http-413` | 11 |
| Error envelope `{error,code,request_id}` everywhere | all |
| 3 routes mounted PUBLIC (no JWT) | 12 |

**Out of scope (later phases):** admin release/policy upsert+delete, admin `/admin/errors` list/get/patch/delete + `/admin/errors/:id/suggest-fix`, the fix-proposal hourly cron, `/admin/inbox*`. The `errors` write path here is fully self-contained without them.
