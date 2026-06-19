package core

import "testing"

func TestUseGoBackend_RampBoundaries(t *testing.T) {
	if UseGoBackend("any-user-id", 0) {
		t.Error("ramp 0 must bucket everyone OUT")
	}
	if !UseGoBackend("any-user-id", 100) {
		t.Error("ramp 100 must bucket everyone IN")
	}
}

func TestUseGoBackend_DeterministicPerUser(t *testing.T) {
	a := UseGoBackend("user-abc", 50)
	b := UseGoBackend("user-abc", 50)
	if a != b {
		t.Error("same user + same ramp must be stable")
	}
}

// TestUseGoBackend_NodeParityVectors pins the Go bucket function to the
// exact output of the Node FeatureFlagsService.bucket() algorithm
// (src/auth/feature-flags.service.ts): SHA-256(userId) -> first 4 bytes
// as BigEndian uint32 -> % 100, compared with strict `<` against the
// ramp. Vectors computed by running the Node algorithm directly:
//
//	bucket("11111111-1111-1111-1111-111111111111") = 32
//	bucket("user-abc")                             = 52
//	bucket("u-1")                                  = 53
//
// If either backend drifts, the same user would get contradictory
// answers mid-migration — these assertions catch that.
func TestUseGoBackend_NodeParityVectors(t *testing.T) {
	cases := []struct {
		userID string
		ramp   int
		want   bool
	}{
		// bucket 32: in at ramp 33 (32<33), out at ramp 32 (32<32 false)
		{"11111111-1111-1111-1111-111111111111", 33, true},
		{"11111111-1111-1111-1111-111111111111", 32, false},
		// bucket 52: out at ramp 50, in at ramp 53
		{"user-abc", 50, false},
		{"user-abc", 53, true},
		// bucket 53: out at ramp 53 (strict <), in at ramp 54
		{"u-1", 53, false},
		{"u-1", 54, true},
	}
	for _, c := range cases {
		if got := UseGoBackend(c.userID, c.ramp); got != c.want {
			t.Errorf("UseGoBackend(%q, %d) = %v, want %v (Node parity)",
				c.userID, c.ramp, got, c.want)
		}
	}
}
