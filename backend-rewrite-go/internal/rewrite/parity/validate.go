package parity

import (
	"encoding/json"
	"strings"
)

// toneInMessage is class-validator's @IsIn message for the `tone` property —
// "tone must be one of the following values: <comma-joined TONE_IDS in
// declaration order>". Built from ToneIDs so the list lives in exactly one
// place (Rule #1 — never hardcode the 8 ids twice).
var toneInMessage = "tone must be one of the following values: " + strings.Join(ToneIDs, ", ")

// resolveString reproduces class-validator's @IsString view of one property:
//   - key absent (nil)                 → ("", isString=false)
//   - present JSON string              → (value, isString=true)
//   - present but any other JSON type  → ("", isString=false)
func resolveString(raw *json.RawMessage) (val string, isString bool) {
	if raw == nil {
		return "", false
	}
	var s string
	if err := json.Unmarshal(*raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// toneIn reports whether tone is one of the canonical ToneIDs.
func toneIn(tone string) bool {
	for _, id := range ToneIDs {
		if id == tone {
			return true
		}
	}
	return false
}

// validateRewrite reproduces the NestJS global ValidationPipe
// ({whitelist:true, forbidNonWhitelisted:true}) over RewriteDto, in
// property-declaration order (text, tone, target_language, source_language):
//
//	@IsString()              text          (NO length cap, NO trim)
//	@IsIn(TONE_IDS)          tone
//	@IsOptional @IsString()  target_language?
//	@IsOptional @IsString()  source_language?
//
// On any failure Node throws BadRequestException(constraintStrings[]),
// AllExceptionsFilter joins them with ". " (each verbatim — none hit a
// humanizeValidation special case) and replies 400 / invalid-input.
//
// forbidNonWhitelisted (unknown-key) messages are appended AFTER the
// known-property messages, in the order the unknown keys appear in the request
// JSON — mirroring the internal/user convention. Single-error inputs (the
// realistic case) are byte-exact; exotic multi-error ordering is pinned by the
// shadow gate and intentionally not over-engineered.
//
// Returns the parsed text/tone/target/source and a combined error message
// (failures joined with ". "); empty msg = valid.
func validateRewrite(raw []byte) (text, tone, target, source, msg string) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		// Parse error, or a non-object/null body. Mirror the unknown-key path's
		// generic phrasing handled by the caller; validateRewrite itself returns
		// a sentinel that the handler maps. We keep this simple: surface the
		// @IsString + @IsIn pair as if the body were empty (text + tone missing).
		return "", "", "", "", "text must be a string. " + toneInMessage
	}

	// field returns the raw message pointer for a key (nil when absent), so
	// resolveString can distinguish missing from present-wrong-type.
	field := func(key string) *json.RawMessage {
		if v, ok := fields[key]; ok {
			return &v
		}
		return nil
	}

	var msgs []string

	// text — @IsString. Missing OR non-string → "text must be a string".
	if t, isStr := resolveString(field("text")); !isStr {
		msgs = append(msgs, "text must be a string")
	} else {
		text = t
	}

	// tone — @IsIn(TONE_IDS). Missing OR not one of the 8 → the IN message.
	if tn, toneIsStr := resolveString(field("tone")); !toneIsStr || !toneIn(tn) {
		msgs = append(msgs, toneInMessage)
	} else {
		tone = tn
	}

	// target_language — @IsOptional @IsString. Only present-but-non-string trips.
	if tl := field("target_language"); tl != nil {
		if v, ok := resolveString(tl); !ok {
			msgs = append(msgs, "target_language must be a string")
		} else {
			target = v
		}
	}

	// source_language — @IsOptional @IsString. Only present-but-non-string trips.
	if sl := field("source_language"); sl != nil {
		if v, ok := resolveString(sl); !ok {
			msgs = append(msgs, "source_language must be a string")
		} else {
			source = v
		}
	}

	// forbidNonWhitelisted: any key outside the whitelist is rejected.
	known := map[string]struct{}{
		"text": {}, "tone": {}, "target_language": {}, "source_language": {},
	}
	for k := range fields {
		if _, ok := known[k]; !ok {
			msgs = append(msgs, "property "+k+" should not exist")
		}
	}

	return text, tone, target, source, strings.Join(msgs, ". ")
}
