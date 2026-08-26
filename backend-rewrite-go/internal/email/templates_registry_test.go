package email

import "testing"

// Every template key must resolve. The keys were previously written twice —
// once as map keys, once as a literal at each call site — so a rename updated
// one copy and the other silently stopped matching, rendering nothing.
func TestBuiltinRegistry_EveryKeyResolves(t *testing.T) {
	reg := BuiltinRegistry()
	keys := []string{
		TemplateVerification, TemplatePasswordReset, TemplateRenewalReminder,
		TemplateSubscriptionActivated, TemplatePaymentFailed, TemplateSubscriptionExpired,
	}
	for _, k := range keys {
		subj, html := reg.Render(k, map[string]string{"name": "X", "code": "123456", "plan": "Pro", "expires": "Mon Jun 15 2026", "amount": "$9.99"})
		if subj == "" || html == "" {
			t.Errorf("template %q rendered empty (subject=%q html-empty=%v)", k, subj, html == "")
		}
	}
	if len(reg) != len(keys) {
		t.Errorf("registry has %d entries but %d keys are declared — one is unreachable", len(reg), len(keys))
	}
}
