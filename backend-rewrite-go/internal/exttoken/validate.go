package exttoken

import (
	"regexp"
	"strings"
)

var (
	// Node validates device_id with class-validator's @IsUUID('4') — strictly
	// a version-4 UUID (13th nibble == 4, 17th nibble in [89abAB]).
	uuid4Re = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	// Node: @Matches(/^[A-Za-z0-9 _.\-]+$/) on device_name. The `+` requires at
	// least one char, so empty device_name fails @Matches too (in addition to
	// @Length) — matching class-validator's per-constraint collection.
	deviceNameRe = regexp.MustCompile(`^[A-Za-z0-9 _.\-]+$`)
)

// ValidateMint reproduces the NestJS ValidationPipe + AllExceptionsFilter for
// MintExtensionTokenDto, returning the single joined message string the Go
// error envelope carries (code invalid-input, status 400), or "" when valid.
//
// Parity contract (byte-for-byte with Node):
//   - Constraints are collected in property-declaration order: device_id
//     (@IsUUID('4')) then device_name (@Length(1,64) then @Matches), exactly
//     as class-validator evaluates them.
//   - Each failing constraint's raw class-validator message is passed through
//     Node's humanizeValidation (all-exceptions.filter.ts) and the results are
//     joined with ". " — the AllExceptionsFilter array-message path.
//
// Raw class-validator default messages (verified against node_modules):
//
//	@IsUUID('4')                 → "device_id must be a UUID"
//	@Length(1,64) too short/empty→ "device_name must be longer than or equal to 1 characters"
//	@Length(1,64) too long       → "device_name must be shorter than or equal to 64 characters"
//	@Matches (custom message)    → "device_name must be alphanumeric/space/_/./- only"
//
// (We deliberately do NOT emit @IsString's "device_name must be a string" —
// the Go decode can't distinguish a missing field from an empty string; this
// mirrors the auth module's validateRegister, which also omits @IsString.)
func ValidateMint(deviceID, deviceName string) string {
	var raws []string

	if !uuid4Re.MatchString(deviceID) {
		raws = append(raws, "device_id must be a UUID")
	}

	if l := len(deviceName); l < 1 {
		raws = append(raws, "device_name must be longer than or equal to 1 characters")
	} else if l > 64 {
		raws = append(raws, "device_name must be shorter than or equal to 64 characters")
	}
	if !deviceNameRe.MatchString(deviceName) {
		// Custom @Matches message — overrides class-validator's default.
		raws = append(raws, "device_name must be alphanumeric/space/_/./- only")
	}

	if len(raws) == 0 {
		return ""
	}
	humanized := make([]string, len(raws))
	for i, raw := range raws {
		humanized[i] = humanizeValidation(raw)
	}
	return strings.Join(humanized, ". ")
}

// humanizeValidation mirrors AllExceptionsFilter.humanizeValidation in
// backend/src/common/all-exceptions.filter.ts: it rewrites a small set of
// class-validator constraint phrases into friendlier copy, leaving anything
// unrecognised verbatim. Only the rules reachable from MintExtensionTokenDto
// are exercised here, but the full rule set is reproduced for fidelity.
func humanizeValidation(raw string) string {
	m := strings.ToLower(raw)
	field := strings.SplitN(raw, " ", 2)[0]
	titleCase := field
	if field != "" {
		titleCase = strings.ToUpper(field[:1]) + field[1:]
	}
	if strings.HasSuffix(m, "must be an email") {
		return "Please enter a valid email address."
	}
	if strings.Contains(m, "must be longer than or equal to") {
		if n := firstNumber(raw); n != "" {
			return titleCase + " must be at least " + n + " characters."
		}
		return raw
	}
	if strings.HasSuffix(m, "should not be empty") {
		return titleCase + " is required."
	}
	return raw
}

var digitsRe = regexp.MustCompile(`\d+`)

// firstNumber returns the first run of digits in s, or "" — mirrors the JS
// `raw.match(/(\d+)/)?.[1]` used by humanizeValidation.
func firstNumber(s string) string {
	return digitsRe.FindString(s)
}
