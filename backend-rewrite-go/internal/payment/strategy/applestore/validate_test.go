package applestore

import "testing"

// TestValidateConfig pins the boot-time fail-fast/fail-safe invariant (final
// whole-branch review finding): Apple IAP must be wired ONLY when both
// bundleID and a recognized environment are set. Exactly one set is a
// misconfiguration that must fail fast, never silently mount a live endpoint
// that accepts Sandbox transactions as Production (see doc comment).
func TestValidateConfig(t *testing.T) {
	cases := []struct {
		name           string
		bundleID       string
		environment    string
		wantConfigured bool
		wantErr        bool
	}{
		{
			name:           "both empty — not configured, skip wiring, no error",
			bundleID:       "",
			environment:    "",
			wantConfigured: false,
			wantErr:        false,
		},
		{
			name:           "both valid (Production) — configured, wire it",
			bundleID:       "com.draftright.app",
			environment:    EnvProduction,
			wantConfigured: true,
			wantErr:        false,
		},
		{
			name:           "both valid (Sandbox) — configured, wire it",
			bundleID:       "com.draftright.app",
			environment:    EnvSandbox,
			wantConfigured: true,
			wantErr:        false,
		},
		{
			name:           "bundle only, environment empty — fail fast",
			bundleID:       "com.draftright.app",
			environment:    "",
			wantConfigured: false,
			wantErr:        true,
		},
		{
			name:           "environment only, bundle empty — fail fast",
			bundleID:       "",
			environment:    EnvProduction,
			wantConfigured: false,
			wantErr:        true,
		},
		{
			name:           "bundle set, environment garbage — fail fast",
			bundleID:       "com.draftright.app",
			environment:    "production", // case mismatch — not the Apple-spec value
			wantConfigured: false,
			wantErr:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configured, err := ValidateConfig(tc.bundleID, tc.environment)
			if configured != tc.wantConfigured {
				t.Errorf("configured = %v, want %v", configured, tc.wantConfigured)
			}
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
