package applestore

import "fmt"

// EnvProduction and EnvSandbox are the only two values Apple's StoreKit JWS
// `environment` claim takes. One source of truth — ValidateConfig and every
// caller compare against these constants instead of re-typing the literals
// (see review finding: an operator-set APPLE_ENVIRONMENT that is neither of
// these must never be treated as "valid").
const (
	EnvProduction = "Production"
	EnvSandbox    = "Sandbox"
)

// ValidateConfig enforces the Apple IAP fail-fast/fail-safe wiring invariant.
//
// It returns configured=true only when both bundleID and a valid environment
// (EnvProduction or EnvSandbox) are set — the composition root should wire
// the Apple strategy, inject the verifier seam, and mount its handlers only
// in that case.
//
// Exactly one of the two set means the operator clearly intends Apple IAP
// but misconfigured it: err is non-nil and the caller MUST fail fast at boot
// rather than silently mount a live endpoint. This matters because
// NewVerifier treats an empty wantEnv as "skip the environment claim check"
// (see Verify) — wiring with bundleID set and environment empty would
// silently accept Sandbox/StoreKit-test transactions as genuine Production
// purchases and grant real Pro for free.
//
// Both empty means Apple IAP simply isn't configured: configured=false,
// err=nil — the existing safe default of mounting no Apple routes at all.
func ValidateConfig(bundleID, environment string) (configured bool, err error) {
	validEnv := environment == EnvProduction || environment == EnvSandbox

	switch {
	case bundleID == "" && environment == "":
		return false, nil
	case bundleID != "" && validEnv:
		return true, nil
	case bundleID == "":
		return false, fmt.Errorf("apple iap misconfigured: APPLE_ENVIRONMENT=%q is set but APPLE_BUNDLE_ID is empty", environment)
	default:
		return false, fmt.Errorf("apple iap misconfigured: APPLE_BUNDLE_ID is set but APPLE_ENVIRONMENT=%q must be %q or %q", environment, EnvProduction, EnvSandbox)
	}
}
