package shared

import "strings"

// MaskMarker is U+2026 HORIZONTAL ELLIPSIS — the single marker MaskSecret
// emits and the write path keys on. Real API keys/secrets are ASCII and never
// contain it.
const MaskMarker = "…"

// MaskSecret renders a stored secret for an admin API response. Empty stays
// empty (unset). Secrets shorter than 16 runes reveal nothing but the marker;
// longer secrets reveal first 3 + marker + last 4. Implemented identically in
// the Node backend (src/common/mask-secret.util.ts) — both sides MUST agree
// byte-for-byte or the Go-vs-Node shadow gate fails. Rune-based indexing (Node
// uses code-point iteration) keeps a future non-ASCII secret from drifting.
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) < 16 {
		return MaskMarker
	}
	return string(r[:3]) + MaskMarker + string(r[len(r)-4:])
}

// ContainsMaskMarker reports whether a secret value being written is a masked
// echo (contains U+2026) and must be ignored on save (keep the stored value).
func ContainsMaskMarker(s string) bool {
	return strings.Contains(s, MaskMarker)
}
