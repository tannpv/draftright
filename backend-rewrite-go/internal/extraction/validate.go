package extraction

import (
	"encoding/json"
	"strings"
)

// requestBody mirrors NestJS ExtractRequestDto (dto/extract.dto.ts). kinds is
// decoded as raw strings so a non-enum value is caught by @IsEnum below rather
// than failing the JSON decode.
//
// text is captured as a raw message (not a plain string) so the handler can
// reproduce Node's @IsString behaviour: a missing key OR a present-but-non-string
// JSON value (number/array/object/null) must surface the class-validator
// "text must be a string" message, NOT a generic JSON-decode failure. A Go
// `string` field would either error the decode (wrong type) or silently default
// (missing) — neither matches Node.
type requestBody struct {
	Text  *json.RawMessage `json:"text"`
	Kinds []string         `json:"kinds"`
}

// resolveText reproduces class-validator's view of the `text` property:
//   - key absent (nil RawMessage)         → ("", isString=false)
//   - present JSON string                 → (value, isString=true)
//   - present but any other JSON type     → ("", isString=false)
//
// The bool feeds the @IsString check in validateExtract; only a real string
// reaches the LLM (and the @MaxLength cap).
func resolveText(raw *json.RawMessage) (text string, isString bool) {
	if raw == nil {
		return "", false
	}
	var s string
	if err := json.Unmarshal(*raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// kindsEnumMessage is class-validator's @IsEnum({each:true}) message for the
// `kinds` array — "each value in <prop> must be one of the following values:
// <comma-joined enum VALUES in declaration order>". allKinds preserves the
// EntityKind declaration order (phone…bankAccount), matching Object.values.
var kindsEnumMessage = "each value in kinds must be one of the following values: " + joinKinds(allKinds)

func joinKinds(ks []EntityKind) string {
	parts := make([]string, len(ks))
	for i, k := range ks {
		parts[i] = string(k)
	}
	return strings.Join(parts, ", ")
}

// validateExtract reproduces the NestJS ValidationPipe + humanizer for
// ExtractRequestDto, in property-declaration order (text, then kinds):
//   - text: @IsString @MaxLength(8000) (UTF-16 units, JS String.length). When
//     `text` is missing or a non-string JSON type, class-validator reports BOTH
//     @MaxLength then @IsString — verified by running it against the real DTO:
//     body {} or {"text":123} → ["text must be shorter than or equal to 8000
//     characters","text must be a string"]. Both messages match no
//     humanizeValidation rewrite, so they pass verbatim, joined by ". ". When
//     `text` IS a string, only the @MaxLength cap can trip.
//   - kinds: @ArrayMaxSize(20) then @IsEnum(EntityKind,{each:true}).
//
// Returns the cleaned text, the parsed (validated) kinds, and a combined error
// message (failures joined with ". "); empty errMsg = valid. Code invalid-input,
// 400.
func validateExtract(b requestBody) (cleanText string, kinds []EntityKind, errMsg string) {
	var msgs []string

	text, isString := resolveText(b.Text)
	if !isString {
		// Missing OR non-string `text`. class-validator emits @MaxLength first,
		// then @IsString (declaration order is @IsString then @MaxLength, but the
		// constraint output orders maxLength before isString — same ordering the
		// `description`/`source` checks in feedback/validate.go rely on).
		msgs = append(msgs, "text must be shorter than or equal to 8000 characters")
		msgs = append(msgs, "text must be a string")
	} else if lenUTF16(text) > 8000 {
		msgs = append(msgs, "text must be shorter than or equal to 8000 characters")
	}

	if len(b.Kinds) > 20 {
		msgs = append(msgs, "kinds must contain no more than 20 elements")
	}
	out := make([]EntityKind, 0, len(b.Kinds))
	badKind := false
	for _, raw := range b.Kinds {
		k := EntityKind(raw)
		if !kindValid(k) {
			badKind = true
			continue
		}
		out = append(out, k)
	}
	if badKind {
		msgs = append(msgs, kindsEnumMessage)
	}

	return text, out, strings.Join(msgs, ". ")
}
