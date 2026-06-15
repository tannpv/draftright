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
	"unicode/utf16"
)

// AllowedPlatforms / allowedSeverity mirror the Node service constants.
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

// sliceUTF16 caps s to n UTF-16 code units, mirroring JS String.slice(0, n)
// (which operates on UTF-16 code units, not runes or bytes).
func sliceUTF16(s string, n int) string {
	u := utf16.Encode([]rune(s))
	if len(u) <= n {
		return s
	}
	return string(utf16.Decode(u[:n]))
}

var (
	reBearer   = regexp.MustCompile(`Bearer\s+[a-zA-Z0-9._\-]+`)
	rePassword = regexp.MustCompile(`(?i)password["':\s=]+["']?[^"'\s,}]+`)
	reEmail    = regexp.MustCompile(`[\w._%+\-]+@[\w.\-]+\.[a-zA-Z]{2,}`)
)

// Scrub redacts bearer tokens, passwords, and emails. Mirrors the Node
// service's chained .replace() calls in order.
func Scrub(s string) string {
	s = reBearer.ReplaceAllString(s, "Bearer [REDACTED]")
	s = rePassword.ReplaceAllString(s, "password=[REDACTED]")
	s = reEmail.ReplaceAllString(s, "[email]")
	return s
}

// Fingerprint = sha256hex( errorType + "::" + first3NonEmptyTrimmedStackLines.join("|") ).
// errorType and stackTrace are the ALREADY-SLICED values (200 / 20000).
// Mirrors ErrorsService.fingerprint: split("\n").slice(0,3).map(trim).filter(Boolean).join("|").
func Fingerprint(errorType, stackTrace string) string {
	lines := strings.Split(stackTrace, "\n")
	if len(lines) > 3 {
		lines = lines[:3]
	}
	var frames []string
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			frames = append(frames, ln)
		}
	}
	seed := errorType + "::" + strings.Join(frames, "|")
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}
