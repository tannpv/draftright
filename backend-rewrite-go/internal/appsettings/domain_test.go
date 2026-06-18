package appsettings

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAppSettings_JSONKeyOrder(t *testing.T) {
	s := AppSettings{ID: "1", Environment: "testing", StripeSecretKey: "sk_secret", UpdatedAt: time.Unix(0, 0).UTC()}
	raw, _ := json.Marshal(s)
	order := []string{"id", "environment", "trial_limit", "token_expiry_minutes",
		"refresh_token_expiry_days", "max_input_length", "supported_languages",
		"payment_methods_enabled", "stripe_secret_key", "stripe_webhook_secret",
		"stripe_mode", "paypal_client_id", "paypal_client_secret", "paypal_mode",
		"momo_partner_code", "momo_access_key", "momo_secret_key", "momo_mode",
		"vietqr_bank_id", "vietqr_account_number", "vietqr_account_name",
		"casso_api_key", "sepay_api_key", "sepay_mode", "resend_api_key", "email_from",
		"google_client_id", "google_client_secret", "apple_client_id", "apple_team_id",
		"apple_key_id", "lemonsqueezy_api_key", "lemonsqueezy_store_id",
		"lemonsqueezy_webhook_secret", "lemonsqueezy_variant_monthly",
		"lemonsqueezy_variant_yearly", "client_log_level", "updated_at"}
	prev := -1
	for _, k := range order {
		idx := strings.Index(string(raw), `"`+k+`"`)
		if idx <= prev {
			t.Fatalf("key %q out of order: %s", k, raw)
		}
		prev = idx
	}
	if !strings.Contains(string(raw), `"stripe_secret_key":"sk_secret"`) {
		t.Fatalf("secrets must be unmasked: %s", raw)
	}
}

func TestAppSettings_UpdatedAtISOMillis(t *testing.T) {
	s := AppSettings{UpdatedAt: time.Unix(0, 0).UTC()}
	raw, _ := json.Marshal(s)
	if !strings.Contains(string(raw), `"updated_at":"1970-01-01T00:00:00.000Z"`) {
		t.Fatalf("updated_at must render via shared.ISOMillis: %s", raw)
	}
}
