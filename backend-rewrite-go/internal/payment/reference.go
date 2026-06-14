package payment

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

// PaymentRefPrefix is the literal prefix on every reference code. Customers
// put the full code in the bank-transfer memo; the SePay/Casso/MB webhook
// (Phase 3c) matches it back. Generator + matcher MUST agree — both live here.
// Ports payment-reference.ts PAYMENT_REF_PREFIX.
const PaymentRefPrefix = "DR-PRO-"

// paymentRefRegex matches any DraftRight reference (DR-<SECTION>-<ALNUM>) in a
// transfer memo. Ports PAYMENT_REF_REGEX. Applied to upper-cased text.
var paymentRefRegex = regexp.MustCompile(`DR-[A-Z]+-[A-Z0-9]+`)

// GeneratePaymentReference returns DR-PRO-XXXXXXXX where XXXXXXXX is 4 random
// bytes as upper-case hex. Ports generatePaymentReference()
// (randomBytes(4).toString('hex').toUpperCase()).
func GeneratePaymentReference() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is catastrophic and unrecoverable; mirror Node,
		// which would throw. Panicking here is correct — a payment without a
		// unique reference must never be created.
		panic("payment: crypto/rand failed: " + err.Error())
	}
	return PaymentRefPrefix + strings.ToUpper(hex.EncodeToString(b[:]))
}

// ExtractPaymentReference pulls the first DraftRight reference out of free-text
// transfer content (case-insensitive). Returns "" when none is present.
// Ports extractPaymentReference().
func ExtractPaymentReference(text string) string {
	return paymentRefRegex.FindString(strings.ToUpper(text))
}
