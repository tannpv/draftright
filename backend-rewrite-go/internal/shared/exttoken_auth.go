package shared

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

// extTokenPrefix mirrors exttoken.TokenPrefix ("dr_ext_"). It is
// re-declared here, not imported, on purpose: the exttoken package
// imports shared (its handler uses shared.WriteError), so shared
// importing exttoken would be an import cycle. The ExtVerifier port
// below is declared here for the same reason — the consumer owns the
// interface (Rule: accept interfaces). Keep this constant in sync with
// exttoken.TokenPrefix; a domain_test in exttoken pins that value.
const extTokenPrefix = "dr_ext_"

// bearerScheme is the exact prefix Node's RewriteAuthGuard tests for
// (`header.startsWith('Bearer ')`) and slices off (`header.slice(7)`).
// Keep the trailing space — the raw remainder after it is hashed by the
// ext verifier verbatim, so trimming here would diverge from Node.
const bearerScheme = "Bearer "

// missingBearerMsg mirrors exttoken.ErrMissingToken.Error() ==
// "Missing bearer token". Declared locally because shared must never
// import exttoken (import cycle). Node throws this verbatim from
// RewriteAuthGuard when the header is absent or not a Bearer header,
// BEFORE the ext/JWT split — so it is produced here, not delegated to
// the JWT middleware (which would emit the generic "Unauthorized").
const missingBearerMsg = "Missing bearer token"

// ExtVerifier is the consumer-side port for extension-token
// verification. Satisfied structurally by *exttoken.Service.Verify.
// The returned error's Error() string is surfaced verbatim as the 401
// message, byte-for-byte matching Node's UnauthorizedException text
// ("Invalid extension token" / "Token missing rewrite scope" /
// "Missing bearer token").
type ExtVerifier interface {
	Verify(ctx context.Context, raw string) (userID string, err error)
}

// ExtOrJWT guards /v1/rewrite with dual auth, mirroring Node's
// RewriteAuthGuard exactly:
//
//  1. Header missing OR not prefixed "Bearer " → 401 "Missing bearer
//     token". Node throws this from the guard itself BEFORE the ext/JWT
//     split, so it is reproduced here (NOT delegated to the JWT
//     middleware, which would emit the generic "Unauthorized"). The
//     decision keys on the RAW header exactly like
//     `header.startsWith('Bearer ')`.
//  2. token := header[len("Bearer "):] — the RAW remainder, matching
//     `header.slice('Bearer '.length)`. NO TrimSpace: a trailing-space
//     ext token must reach the verifier with the space intact so its
//     hash matches Node's (which hashes the raw slice).
//  3. token prefixed dr_ext_ (and ext wired) → verified as an extension
//     token; on success the resolved owner is injected as
//     auth.Claims{Sub: uid} under the same context key RequireAuth
//     uses, so the rewrite handler is agnostic to which path
//     authenticated. On failure the verifier's sentinel message is
//     surfaced as a 401.
//  4. Any other token — INCLUDING empty "" or whitespace-only, or a
//     dr_ext_ token when ext is nil — is delegated UNCHANGED to the jwt
//     middleware (the RequireAuth-wrapped handler), preserving
//     byte-identical JWT behavior: it 401s "Unauthorized" on a
//     bad/empty/blank token. This mirrors Node's `super.canActivate`
//     fallthrough, where an empty token after "Bearer " is handed to
//     JwtAuthGuard and rejected as "Unauthorized".
//
// jwt is the already-built RequireAuth middleware value applied to next.
func ExtOrJWT(ext ExtVerifier, jwt func(http.Handler) http.Handler, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// The unchanged RequireAuth-wrapped handler for the JWT branch.
		jwtPath := jwt(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, bearerScheme) {
				// Node: no/!Bearer header → UnauthorizedException
				// ('Missing bearer token'). code = inferCode(401) =
				// invalid-token. Match it verbatim.
				WriteError(w, r, "invalid-token", missingBearerMsg)
				return
			}
			// RAW remainder after "Bearer " — matches header.slice(7).
			// Intentionally NOT trimmed (see bearerScheme doc).
			token := header[len(bearerScheme):]
			if ext != nil && strings.HasPrefix(token, extTokenPrefix) {
				uid, err := ext.Verify(r.Context(), token)
				if err != nil {
					// 401 with the sentinel message verbatim; code is
					// invalid-token (Node inferCode(401)). Logged at
					// debug like RequireAuth — auth failures are normal.
					log.Debug("rewrite-auth: extension token rejected", "err", err.Error())
					WriteError(w, r, "invalid-token", err.Error())
					return
				}
				// Inject the resolved owner under the SAME key RequireAuth
				// uses so ClaimsFromContext works downstream. Only Sub is
				// known (and only Sub is read by the rewrite handler);
				// Node's ext path likewise only resolves the user id.
				ctx := ContextWithClaims(r.Context(), &auth.Claims{Sub: uid})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			// Non-ext token (incl. empty/blank, or a dr_ext_ token with
			// no ext service wired) → identical JWT behavior. The JWT
			// middleware rejects an empty/blank/opaque token as
			// "Unauthorized", matching Node's super.canActivate.
			jwtPath.ServeHTTP(w, r)
		})
	}
}
