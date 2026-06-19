// admin_usecase.go — DTO validation for the admin grant route.
//
// Reproduces the NestJS global ValidationPipe ({whitelist:true,
// forbidNonWhitelisted:true, transform:true}) + AllExceptionsFilter over
// GrantSubscriptionDto for POST /admin/subscriptions/grant.
//
// GrantSubscriptionDto (declaration order — user_id, plan_id, expires_at):
//
//	@IsUUID()              user_id   // version-less → class-validator 'all'
//	@IsUUID()              plan_id   // version-less → 'all'
//	@IsOptional @IsDateString expires_at?  // ISO 8601 (alias of @IsISO8601)
//
// On any failure Node's exceptionFactory flatMaps every constraint string and
// the AllExceptionsFilter humanizes each (none of these messages hit a special
// case) and joins them with ". " → 400 / invalid-input.
//
// ORDER (verified empirically against class-validator 0.14.4 + the real DTO):
// the ValidationPipe whitelist pass (forbidNonWhitelisted) runs first and
// prepends its "property X should not exist" errors — one per unknown key, in
// JSON source-insertion order — BEFORE the metadata-driven field errors
// (user_id, plan_id, expires_at) in declaration order. e.g.
// {user_id:'x',plan_id:'y',expires_at:'nope',bogus:1} →
// "property bogus should not exist. user_id must be a UUID. plan_id must be a
// UUID. expires_at must be a valid ISO 8601 date string".
//
// Raw class-validator default messages (verified against node_modules
// class-validator 0.14.4):
//
//	@IsUUID()      → "<property> must be a UUID"
//	@IsDateString  → "<property> must be a valid ISO 8601 date string"
//	forbidNonWhitelisted → "property <key> should not exist"
package subscription

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// class-validator @IsUUID() with no version arg uses validator.js's 'all'
// regex: versions 1-8, the nil UUID, or the max UUID. Case-insensitive.
var uuidAllRe = regexp.MustCompile(`(?i)^(?:[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}|00000000-0000-0000-0000-000000000000|ffffffff-ffff-ffff-ffff-ffffffffffff)$`)

// grantInput is the validated, parsed grant request.
type grantInput struct {
	UserID    string
	PlanID    string
	ExpiresAt *time.Time
}

// validateGrant reproduces the ValidationPipe over GrantSubscriptionDto.
//
// Three outcomes (the caller distinguishes all three):
//   - malformed == true: the body is not a JSON object (parse error, a JSON
//     array/scalar, or literal null → fields == nil). The caller rejects with
//     400 "Invalid request body" (mirrors the user-admin handler).
//   - malformed == false, msg != "": a parsed object failing field validation.
//     The caller rejects with 400 invalid-input + the joined msg.
//   - malformed == false, msg == "": a valid object → in.UserID/PlanID set,
//     in.ExpiresAt parsed (nil when absent).
func validateGrant(raw []byte) (in grantInput, malformed bool, msg string) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return in, true, ""
	}

	known := map[string]struct{}{"user_id": {}, "plan_id": {}, "expires_at": {}}

	var raws []string

	// forbidNonWhitelisted FIRST: the ValidationPipe whitelist pass prepends
	// "property X should not exist" for every unknown key, in JSON
	// source-insertion order, before the field-constraint errors. Go's map
	// iteration is randomized and json.Unmarshal discards insertion order, so
	// scan the raw body's top-level keys in literal source order instead.
	for _, k := range topLevelKeysInOrder(raw) {
		if _, ok := known[k]; !ok {
			raws = append(raws, "property "+k+" should not exist")
		}
	}

	// Known properties next, in declaration order: user_id, plan_id, expires_at.
	in.UserID = uuidField(fields, "user_id", &raws)
	in.PlanID = uuidField(fields, "plan_id", &raws)

	// expires_at: @IsOptional — only validated when present and non-null.
	// @IsDateString is an alias of @IsISO8601; we accept the realistic ISO
	// timestamp / date forms an admin client sends (the same instants
	// `new Date(dto.expires_at)` yields). A value that parses is accepted and
	// captured; anything else fails the @IsDateString constraint.
	if v, ok := fields["expires_at"]; ok && string(v) != "null" {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			raws = append(raws, "expires_at must be a valid ISO 8601 date string")
		} else if t, perr := parseISO8601(s); perr != nil {
			raws = append(raws, "expires_at must be a valid ISO 8601 date string")
		} else {
			in.ExpiresAt = &t
		}
	}

	if len(raws) == 0 {
		return in, false, ""
	}
	humanized := make([]string, len(raws))
	for i, r := range raws {
		humanized[i] = humanizeValidation(r)
	}
	return grantInput{}, false, strings.Join(humanized, ". ")
}

// topLevelKeysInOrder returns the top-level object keys of raw in their literal
// JSON source order (json.Unmarshal into a map discards this). raw is assumed to
// be a well-formed JSON object — the caller already unmarshalled it. A decode
// hiccup yields a nil slice (the field-error path still fires). Nested
// objects/arrays are skipped so only depth-1 keys are captured.
func topLevelKeysInOrder(raw []byte) []string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	// Opening '{'.
	if _, err := dec.Token(); err != nil {
		return nil
	}
	var keys []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return keys
		}
		key, ok := t.(string)
		if !ok {
			return keys
		}
		keys = append(keys, key)
		// Consume the value. Decoder.Token streams nested delimiters, so a
		// composite value (object/array) advances the depth counter and we
		// drain until it returns to the top level.
		if err := skipValue(dec); err != nil {
			return keys
		}
	}
	return keys
}

// skipValue consumes exactly one JSON value from dec, descending through nested
// objects/arrays via the delimiter depth counter.
func skipValue(dec *json.Decoder) error {
	depth := 0
	for {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := t.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
		if depth == 0 {
			return nil
		}
	}
}

// uuidField validates one required @IsUUID() property, appending the
// class-validator message on failure and returning the trimmed string value.
func uuidField(fields map[string]json.RawMessage, name string, raws *[]string) string {
	v, ok := fields[name]
	if !ok {
		// Missing required property: class-validator runs @IsUUID() on
		// undefined (not a string) → fails.
		*raws = append(*raws, name+" must be a UUID")
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil || !uuidAllRe.MatchString(s) {
		*raws = append(*raws, name+" must be a UUID")
		return ""
	}
	return s
}

// parseISO8601 parses the date forms an admin client realistically sends, the
// same instants `new Date(dto.expires_at)` would yield in Node. Tries
// RFC3339 (with/without sub-seconds, with offset/Z) then date-only (midnight
// UTC, matching JS `new Date('2026-12-31')`).
func parseISO8601(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	var zero time.Time
	return zero, errBadDate
}

var errBadDate = &parseError{}

type parseError struct{}

func (*parseError) Error() string { return "unparseable ISO 8601 date" }

// humanizeValidation mirrors AllExceptionsFilter.humanizeValidation in
// backend/src/common/all-exceptions.filter.ts: it rewrites a small set of
// class-validator phrases into friendlier copy, leaving anything unrecognised
// verbatim. None of GrantSubscriptionDto's messages (UUID / ISO 8601 date /
// whitelist) hit a special case, so each passes through unchanged — but the
// full rule set is reproduced for fidelity with the canonical filter.
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
		if n := digitsRe.FindString(raw); n != "" {
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
