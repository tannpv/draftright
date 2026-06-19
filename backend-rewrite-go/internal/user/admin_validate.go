package user

import (
	"encoding/json"
	"strings"
)

// validateUpdateUser reproduces the NestJS global ValidationPipe
// ({whitelist:true, forbidNonWhitelisted:true}) over UpdateUserDto for
// PATCH /admin/users/:id. UpdateUserDto declares (in order) is_active,
// role, name — all @IsOptional:
//
//	@IsBoolean() is_active?
//	@IsIn(['user']) role?      // NOT @IsString — only the IN check
//	@IsString()  name?
//
// On any failure Node throws BadRequestException(constraintStrings[]),
// AllExceptionsFilter joins them with ". " (each verbatim — none of these
// hit humanizeValidation's special cases) and replies 400 / invalid-input.
//
// Parity decisions:
//   - The body is decoded into map[string]json.RawMessage (NOT the typed
//     struct) so a type mismatch like {"is_active":"x"} surfaces as the
//     class-validator message, not a Go decode error.
//   - Known-property constraint messages are emitted in DECLARATION order
//     (is_active, role, name), matching how NestJS iterates dto properties.
//   - forbidNonWhitelisted (unknown-key) messages are appended AFTER the
//     known-property messages, in the order the unknown keys appear in the
//     request JSON. Single-error inputs (the realistic case) are therefore
//     byte-exact; exotic multi-error ordering is pinned separately by the
//     shadow gate and intentionally not over-engineered here.
//
// Three outcomes (the caller must distinguish all three):
//   - malformed == true: the body is not a JSON object (parse error, a JSON
//     array/scalar, or literal null → fields == nil). The caller rejects with
//     400 "Invalid request body". This restores the pre-c8c32f75 behavior; the
//     earlier code collapsed this into the success path, turning a 400 into a
//     silent 200 no-op write (a regression). 🟡 Node's Express body-parser
//     SyntaxError → 500 internal while we reply 400 here — a separate
//     pre-existing parity gap, intentionally NOT closed in this fix.
//   - malformed == false, msg != "": a parsed object that fails field
//     validation. The caller rejects with 400 invalid-input + the joined msg.
//   - malformed == false, msg == "": a valid object (incl. empty {} — all DTO
//     fields optional) → the caller proceeds with patch.
func validateUpdateUser(raw []byte) (patch UserPatchAdmin, malformed bool, msg string) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		// Parse error, or a non-object/null body (null unmarshals into a nil map
		// with no error). Either way: not a valid UpdateUserDto object.
		return patch, true, ""
	}

	known := map[string]struct{}{"is_active": {}, "role": {}, "name": {}}

	var msgs []string

	// Known properties first, in declaration order (is_active, role, name).
	if v, ok := fields["is_active"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			msgs = append(msgs, "is_active must be a boolean value")
		} else {
			patch.IsActive = &b
		}
	}
	if v, ok := fields["role"]; ok {
		// @IsIn(['user']) — NOT @IsString. class-validator's IN check fails
		// for anything !== "user" (wrong type included), emitting this one
		// message. So: accept only the exact JSON string "user".
		var s string
		if err := json.Unmarshal(v, &s); err != nil || s != "user" {
			msgs = append(msgs, "role must be one of the following values: user")
		} else {
			patch.Role = &s
		}
	}
	if v, ok := fields["name"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			msgs = append(msgs, "name must be a string")
		} else {
			patch.Name = &s
		}
	}

	// forbidNonWhitelisted: any key outside the whitelist is rejected.
	for k := range fields {
		if _, ok := known[k]; !ok {
			msgs = append(msgs, "property "+k+" should not exist")
		}
	}

	if len(msgs) > 0 {
		return UserPatchAdmin{}, false, strings.Join(msgs, ". ")
	}
	return patch, false, ""
}
