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

// AppRelease is the full app_releases row returned by the admin
// listAll/upsert/delete routes. JSON key order mirrors the Node entity
// (src/updates/entities/app-release.entity.ts) exactly. updated_at is a
// Date.toISOString() string (TypeORM serializes timestamps that way).
type AppRelease struct {
	Platform     string `json:"platform"`
	Channel      string `json:"channel"`
	Version      string `json:"version"`
	DownloadURL  string `json:"download_url"`
	SHA256       string `json:"sha256"`
	ReleaseNotes string `json:"release_notes"`
	Required     bool   `json:"required"`
	Enabled      bool   `json:"enabled"`
	UpdatedAt    string `json:"updated_at"`
}

// AppReleasePolicy is the full app_release_policies row returned by the
// admin listAll/upsert routes. JSON key order mirrors the Node entity
// (src/updates/entities/app-release-policy.entity.ts) exactly.
type AppReleasePolicy struct {
	Platform    string `json:"platform"`
	Preferred   string `json:"preferred"`
	StoreStatus string `json:"store_status"`
	Notes       string `json:"notes"`
	UpdatedAt   string `json:"updated_at"`
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
	return parseIntPrefix(parts[i])
}

// parseIntPrefix mirrors JS parseInt(s, 10) || 0: trim leading spaces,
// allow one optional +/- sign, consume the leading digit run, ignore the
// rest; no digits → 0.
func parseIntPrefix(s string) int {
	s = strings.TrimLeft(s, " \t\n\r\f\v")
	j := 0
	if j < len(s) && (s[j] == '+' || s[j] == '-') {
		j++
	}
	start := j
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if start == j { // no digits after optional sign
		return 0
	}
	n, err := strconv.Atoi(s[:j])
	if err != nil {
		return 0
	}
	return n
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
