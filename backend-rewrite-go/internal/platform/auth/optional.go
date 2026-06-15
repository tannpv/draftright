package auth

import (
	"net/http"
	"strings"
)

// OptionalUserID decodes an optional Bearer JWT and returns the user id
// (Claims.UserID), or "" for anonymous/missing/invalid. Never errors.
//
// Mirrors NestJS JwtUserService.decodeOptional (bug-reports/jwt-user.service.ts)
// and the original errreport.optionalUserID: a case-sensitive "Bearer "
// prefix, raw slice(7), and any verify error mapped to "". A nil verifier
// (tests / unconfigured) also yields "".
//
// Shared by errreport, bug_reports, and feedback — all of which read an
// optional bearer for attribution (Rule #1: extend, don't fork). The
// Verifier passed MUST be the access-token verifier, never the refresh one.
func OptionalUserID(v *Verifier, r *http.Request) string {
	if v == nil {
		return ""
	}
	const p = "Bearer "
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(authz, p) {
		return ""
	}
	claims, err := v.Verify(authz[len(p):])
	if err != nil {
		return ""
	}
	return claims.UserID()
}
