package payment

import (
	"fmt"
	"strings"
)

// DefaultPaymentMethod is used when neither app_settings.payment_methods_enabled
// nor env PAYMENT_ENABLED_METHODS is set. Ports DEFAULT_PAYMENT_METHOD.
const DefaultPaymentMethod = string(MethodStripe)

// registeredMethods is the ordered key set of Node's strategy Map
// (payment.service.ts constructor). These are the methods that actually have a
// backend strategy and can therefore check out. paypal + momo are enum values
// with NO strategy and are deliberately absent. 3b's strategies must cover
// EXACTLY this set; keep it the single source of truth.
var registeredMethods = []string{
	string(MethodStripe),
	string(MethodVietQR),
	string(MethodBankTransfer),
	string(MethodLemonSqueezy),
	string(MethodApplePay),
	string(MethodGooglePay),
}

// RegisteredMethods returns the registerable method keys (a fresh copy so
// callers can't mutate the package state).
func RegisteredMethods() []string {
	out := make([]string, len(registeredMethods))
	copy(out, registeredMethods)
	return out
}

// EnabledMethods resolves the storefront's visible methods from a raw CSV
// (already chosen by the caller: settings ?? env ?? default), then applies
// lowercase/dedupe/first-seen ordering and the vietqr→bank_transfer rule.
// Output order is byte-identical to Node's `[...set]`.
func EnabledMethods(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		raw = DefaultPaymentMethod
	}
	raw = strings.ToLower(raw)
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		m := strings.TrimSpace(part)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	if seen[string(MethodVietQR)] && !seen[string(MethodBankTransfer)] {
		seen[string(MethodBankTransfer)] = true
		out = append(out, string(MethodBankTransfer))
	}
	return out
}

// AssertMethodsRegisterable validates a payment_methods_enabled CSV: every
// listed method must have a registered strategy. Blank input is allowed (falls
// back to default). Ports assertMethodsRegisterable() incl. the exact message.
// Does NOT dedupe the unknown list (matches Node).
func AssertMethodsRegisterable(csv string) error {
	registered := map[string]bool{}
	for _, m := range registeredMethods {
		registered[m] = true
	}
	var unknown []string
	for _, part := range strings.Split(strings.ToLower(csv), ",") {
		m := strings.TrimSpace(part)
		if m == "" || registered[m] {
			continue
		}
		unknown = append(unknown, m)
	}
	if len(unknown) > 0 {
		return fmt.Errorf(
			"Cannot enable payment method(s) with no backend strategy: %s. Registered methods: %s.",
			strings.Join(unknown, ", "), strings.Join(registeredMethods, ", "),
		)
	}
	return nil
}
