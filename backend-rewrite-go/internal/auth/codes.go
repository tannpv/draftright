package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// emailCodeTTL mirrors EMAIL_CODE_TTL_MS — 15 minutes.
const emailCodeTTL = 15 * time.Minute

// maxResetAttempts mirrors AuthService.MAX_RESET_ATTEMPTS.
const maxResetAttempts = 5

// generateCode returns a zero-padded 6-digit CSPRNG code, exactly Node's
// randomInt(0, 1_000_000).toString().padStart(6, '0').
func generateCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic("auth: csprng unavailable: " + err.Error())
	}
	return fmt.Sprintf("%06d", n.Int64())
}
