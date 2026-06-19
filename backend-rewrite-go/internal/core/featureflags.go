package core

import (
	"crypto/sha256"
	"encoding/binary"
)

// UseGoBackend reproduces the Node FeatureFlagsService.useGoBackend
// bucket exactly (backend/src/auth/feature-flags.service.ts), so a
// given user gets the same answer from either backend mid-migration:
//
//   - ramp <= 0   -> false (no users on the Go path)
//   - ramp >= 100 -> true  (all users)
//   - otherwise   -> bucket(userID) < ramp  (strict less-than)
//
// bucket(userID) = uint32(BigEndian, first 4 bytes of SHA-256(userID)) % 100,
// matching Node's createHash('sha256').update(id).digest().readUInt32BE(0) % 100.
func UseGoBackend(userID string, rampPercent int) bool {
	if rampPercent <= 0 {
		return false
	}
	if rampPercent >= 100 {
		return true
	}
	return bucket(userID) < rampPercent
}

// bucket assigns a stable 0-99 slot to a user id. Identical to the
// Node bucket(): SHA-256 digest, first 4 bytes as a big-endian uint32,
// modulo 100.
func bucket(userID string) int {
	sum := sha256.Sum256([]byte(userID))
	n := binary.BigEndian.Uint32(sum[:4])
	return int(n % 100)
}
