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
// Returns the validated patch and an empty string on success; a non-empty
// string (the joined message) means reject with 400 invalid-input.
func validateUpdateUser(raw []byte) (UserPatchAdmin, string) {
	var patch UserPatchAdmin

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		// Malformed / non-object JSON: not a validation concern. The caller
		// keeps the pre-existing handling. 🟡 Node's Express body-parser
		// SyntaxError → 500 internal, while the Go handler currently replies
		// 400 "Invalid request body" — a separate pre-existing parity gap,
		// deliberately left untouched by this fix.
		return patch, ""
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
		return UserPatchAdmin{}, strings.Join(msgs, ". ")
	}
	return patch, ""
}
