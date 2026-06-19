// Package errreport ingests client crash reports (POST /errors): optional
// JWT attribution, honeypot drop, sha256 fingerprint dedup, PII scrub.
// Byte-identical port of the NestJS errors module ingest path. Admin
// read/list + the fix-proposal cron are out of scope (later phase).
package errreport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode/utf16"
)

// ErrNotFound is returned by the admin repo when an error_reports row is
// absent. The HTTP edge translates it to 400 "not found" (errors do NOT
// use 404 — see spec §4 / Node BadRequestException('not found')).
var ErrNotFound = errors.New("error report not found")

// ErrorReportEntity is the full error_reports row returned by the admin
// read/list/patch/delete routes. JSON key order mirrors the Node entity
// (src/errors/entities/error-report.entity.ts) exactly so the marshalled
// bytes match. MarshalJSON pins the field order + renders the bigint and
// timestamps the way TypeORM does over the `pg` driver:
//
//   - display_no is a TypeORM bigint → the `pg` driver returns it as a JS
//     STRING (no setTypeParser override in the Node codebase), so JSON.stringify
//     emits "display_no":"203" (quoted). Held as a Go string, never int64.
//   - first_seen_at / last_seen_at / resolved_at are TypeORM Date columns →
//     serialized via Date.toISOString() (always UTC, exactly 3 fractional
//     digits, trailing Z). Held as ISOMillis-formatted strings.
//
// context is jsonb marshalled raw so null/{} round-trip byte-identically.
type ErrorReportEntity struct {
	ID            string          `json:"id"`
	DisplayNo     string          `json:"display_no"`
	Platform      string          `json:"platform"`
	AppVersion    *string         `json:"app_version"`
	Severity      string          `json:"severity"`
	ErrorType     *string         `json:"error_type"`
	Message       *string         `json:"message"`
	StackTrace    *string         `json:"stack_trace"`
	Context       json.RawMessage `json:"context"`
	UserID        *string         `json:"user_id"`
	DeviceID      *string         `json:"device_id"`
	Fingerprint   string          `json:"fingerprint"`
	Count         int             `json:"count"`
	Status        int             `json:"status"`
	AiFixProposal *string         `json:"ai_fix_proposal"`
	ResolvedBy    *string         `json:"resolved_by"`
	ResolvedAt    *string         `json:"resolved_at"`
	FirstSeenAt   string          `json:"first_seen_at"`
	LastSeenAt    string          `json:"last_seen_at"`
}

// AdminListFilter carries the optional filters + pagination for the admin
// list route (Node ErrorsService.list opts). Nil pointers = no filter.
type AdminListFilter struct {
	Platform *string
	Severity *string
	Status   *int
	Limit    int
	Offset   int
}

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

// lenUTF16 returns the length of s in UTF-16 code units, mirroring JS
// String.length (what class-validator's @MaxLength measures), NOT byte
// or rune length. Same encoding sliceUTF16 uses for field capping.
func lenUTF16(s string) int {
	return len(utf16.Encode([]rune(s)))
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
