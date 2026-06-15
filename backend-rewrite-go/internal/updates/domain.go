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
