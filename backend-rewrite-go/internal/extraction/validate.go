package extraction

import "strings"

// requestBody mirrors NestJS ExtractRequestDto (dto/extract.dto.ts). kinds is
// decoded as raw strings so a non-enum value is caught by @IsEnum below rather
// than failing the JSON decode.
type requestBody struct {
	Text  string   `json:"text"`
	Kinds []string `json:"kinds"`
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
//   - text: @MaxLength(8000) (UTF-16 units, JS String.length). The default
//     class-validator message "<prop> must be shorter than or equal to <N>
//     characters" matches no humanizeValidation rewrite, so it passes verbatim.
//   - kinds: @ArrayMaxSize(20) then @IsEnum(EntityKind,{each:true}).
//
// Returns the cleaned text, the parsed (validated) kinds, and a combined error
// message (failures joined with ". "); empty errMsg = valid. Code invalid-input,
// 400.
func validateExtract(b requestBody) (cleanText string, kinds []EntityKind, errMsg string) {
	var msgs []string

	if lenUTF16(b.Text) > 8000 {
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

	return b.Text, out, strings.Join(msgs, ". ")
}
