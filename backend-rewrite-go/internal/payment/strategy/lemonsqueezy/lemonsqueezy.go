// Package lemonsqueezy implements the LemonSqueezy (Merchant-of-Record) payment
// strategy via the JSON:API at api.lemonsqueezy.com/v1. Checkout creates a
// hosted checkout URL; portal/cancel call the customer + subscription APIs.
//
// Why MoR: the business operates from Vietnam, where Stripe isn't available and
// a local card-acquiring contract needs a registered company. LemonSqueezy
// sells the subscription on our behalf and pays out to a VN bank, so we can
// accept cards today. Ports backend/src/payment/strategies/lemonsqueezy.strategy.ts.
package lemonsqueezy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

const defaultAPIBase = "https://api.lemonsqueezy.com/v1"

// Creds is the LemonSqueezy credential set (admin-editable AppSettings in Node).
type Creds struct {
	APIKey         string
	StoreID        string
	VariantMonthly string
	VariantYearly  string
}

// Strategy implements strategy.Strategy for LemonSqueezy.
type Strategy struct {
	creds      Creds
	websiteURL string
	apiBase    string
	http       *http.Client
}

var _ strategy.Strategy = (*Strategy)(nil)

// New builds a LemonSqueezy strategy. websiteURL falls back to the dev default
// when empty (mirrors websiteUrl() in Node).
func New(c Creds, websiteURL string) *Strategy {
	if websiteURL == "" {
		websiteURL = "http://localhost:4000"
	}
	return &Strategy{creds: c, websiteURL: websiteURL, apiBase: defaultAPIBase, http: http.DefaultClient}
}

func (s *Strategy) CreateCheckout(ctx context.Context, p strategy.Payment, plan strategy.Plan, opts strategy.Options) (strategy.Result, error) {
	if s.creds.APIKey == "" || s.creds.StoreID == "" {
		return strategy.Result{}, fmt.Errorf("Lemon Squeezy is not configured. Set the API key + store ID in admin Settings → Payment.")
	}

	isYearly := plan.BillingPeriod == "yearly"
	variantID := s.creds.VariantMonthly
	if isYearly {
		variantID = s.creds.VariantYearly
	}
	if variantID == "" {
		period := "monthly"
		if isYearly {
			period = "yearly"
		}
		return strategy.Result{}, fmt.Errorf(
			"No Lemon Squeezy variant configured for %s billing. Set lemonsqueezy_variant_%s in admin Settings → Payment.",
			period, period,
		)
	}
	variantNum, err := strconv.Atoi(variantID)
	if err != nil {
		return strategy.Result{}, fmt.Errorf("invalid LemonSqueezy variant id: %q", variantID)
	}

	redirectURL := opts.SuccessURL
	if redirectURL == "" {
		redirectURL = fmt.Sprintf("%s/payment/success?ref=%s", s.websiteURL, p.ReferenceCode)
	}

	body := map[string]any{
		"data": map[string]any{
			"type": "checkouts",
			"attributes": map[string]any{
				"checkout_data": map[string]any{
					"email":  opts.UserEmail,
					"custom": map[string]any{"reference_code": p.ReferenceCode},
				},
				"product_options": map[string]any{
					"redirect_url":     redirectURL,
					"enabled_variants": []int{variantNum},
				},
			},
			"relationships": map[string]any{
				"store":   map[string]any{"data": map[string]any{"type": "stores", "id": s.creds.StoreID}},
				"variant": map[string]any{"data": map[string]any{"type": "variants", "id": variantID}},
			},
		},
	}

	var out struct {
		Data struct {
			Attributes struct {
				URL string `json:"url"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := s.do(ctx, http.MethodPost, "/checkouts", body, &out); err != nil {
		return strategy.Result{}, err
	}
	if out.Data.Attributes.URL == "" {
		return strategy.Result{}, fmt.Errorf("Lemon Squeezy checkout returned no URL")
	}
	return strategy.Result{RedirectURL: out.Data.Attributes.URL}, nil
}

func (s *Strategy) CustomerPortalURL(ctx context.Context, u strategy.PortalUser) (string, error) {
	if u.LemonSqueezyCustomerID == "" {
		return "", nil
	}
	if s.creds.APIKey == "" {
		return "", fmt.Errorf("Lemon Squeezy is not configured.")
	}
	var out struct {
		Data struct {
			Attributes struct {
				URLs struct {
					CustomerPortal string `json:"customer_portal"`
				} `json:"urls"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := s.do(ctx, http.MethodGet, "/customers/"+u.LemonSqueezyCustomerID, nil, &out); err != nil {
		return "", err
	}
	return out.Data.Attributes.URLs.CustomerPortal, nil
}

func (s *Strategy) CancelSubscription(ctx context.Context, subscriptionID string) (bool, error) {
	if s.creds.APIKey == "" {
		return false, fmt.Errorf("Lemon Squeezy is not configured.")
	}
	if err := s.do(ctx, http.MethodDelete, "/subscriptions/"+subscriptionID, nil, nil); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Strategy) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.apiBase+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+s.creds.APIKey)
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("lemonsqueezy %s %s: status %d", method, path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
