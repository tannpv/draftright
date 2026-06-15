// Package vietqr implements the VietQR + bank_transfer payment strategy:
// a static img.vietqr.io QR URL + a manual bank-transfer info block. No
// external API call, no secrets at checkout time (webhook keys are Phase 3c).
package vietqr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// Creds are the already-resolved (DB?env?default) display credentials.
type Creds struct {
	BankID        string
	AccountNumber string
	AccountName   string
	CassoAPIKey   string
	SepayAPIKey   string
}

// Strategy is the VietQR/bank-transfer provider.
type Strategy struct{ creds Creds }

// New applies the same local-dev defaults as Node getCredentials():
// MB / 0000000000 / DRAFTRIGHT when a field is empty.
func New(c Creds) *Strategy {
	if c.BankID == "" {
		c.BankID = "MB"
	}
	if c.AccountNumber == "" {
		c.AccountNumber = "0000000000"
	}
	if c.AccountName == "" {
		c.AccountName = "DRAFTRIGHT"
	}
	return &Strategy{creds: c}
}

// bankNames ports getBankDisplayName's table; unknown id → the id itself.
var bankNames = map[string]string{
	"MB":   "MB Bank (Quân Đội)",
	"VCB":  "Vietcombank",
	"ACB":  "ACB",
	"TCB":  "Techcombank",
	"VPB":  "VPBank",
	"TPB":  "TPBank",
	"BIDV": "BIDV",
	"VTB":  "VietinBank",
	"SCB":  "Sacombank",
}

func bankDisplayName(id string) string {
	if n, ok := bankNames[id]; ok {
		return n
	}
	return id
}

// encodeURIComponent fixes url.QueryEscape's one divergence from JS that
// matters here: space encodes as %20, not +. Note QueryEscape uses
// form-urlencoded rules and percent-encodes some chars encodeURIComponent
// leaves literal — fine for our inputs (reference codes are [A-Z0-9-];
// accountName is the only variable field), but not a general equivalent.
func encodeURIComponent(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func (s *Strategy) CreateCheckout(_ context.Context, p strategy.Payment, _ strategy.Plan, _ strategy.Options) (strategy.Result, error) {
	bank := &strategy.BankInfo{
		BankName:      bankDisplayName(s.creds.BankID),
		AccountNumber: s.creds.AccountNumber,
		AccountName:   s.creds.AccountName,
		Amount:        p.Amount,
		Currency:      "VND",
		Reference:     p.ReferenceCode,
	}
	// bank_transfer → manual flow, no QR (client renders the table only).
	if p.Method == "bank_transfer" {
		return strategy.Result{BankInfo: bank}, nil
	}
	qr := fmt.Sprintf(
		"https://img.vietqr.io/image/%s-%s-compact2.jpg?amount=%d&addInfo=%s&accountName=%s",
		s.creds.BankID, s.creds.AccountNumber, p.Amount,
		encodeURIComponent(p.ReferenceCode), encodeURIComponent(s.creds.AccountName),
	)
	return strategy.Result{QRData: qr, BankInfo: bank}, nil
}

// CustomerPortalURL / CancelSubscription: VietQR has neither (default behavior).
func (s *Strategy) CustomerPortalURL(_ context.Context, _ strategy.PortalUser) (string, error) {
	return "", nil
}

func (s *Strategy) CancelSubscription(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// drRefRegex mirrors internal/payment/reference.go's matcher. Duplicated here
// (not imported) to avoid a payment→strategy→payment import cycle; both derive
// from the same Node PAYMENT_REF_REGEX = /DR-[A-Z]+-[A-Z0-9]+/.
var drRefRegex = regexp.MustCompile(`DR-[A-Z]+-[A-Z0-9]+`)

func extractRef(text string) string {
	return drRefRegex.FindString(strings.ToUpper(text))
}

// VerifyWebhook ports VietQRStrategy.verifyWebhook. Header auth first (401 on
// any failure), then reference extraction from the Casso / SePay / MB body
// shapes. Authenticated-but-unmatched → Ignored.
func (s *Strategy) VerifyWebhook(ctx context.Context, payload []byte, headers http.Header) (strategy.WebhookAction, error) {
	authHeader := headers.Get("Authorization")
	secureToken := strings.TrimSpace(headers.Get("Secure-Token"))
	provided := strings.TrimSpace(stripApikeyPrefix(authHeader))
	if provided == "" {
		provided = secureToken
	}
	if provided == "" {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 401, Message: "Missing webhook authorization"}
	}

	var validKeys []string
	if s.creds.CassoAPIKey != "" {
		validKeys = append(validKeys, s.creds.CassoAPIKey)
	}
	if s.creds.SepayAPIKey != "" {
		validKeys = append(validKeys, s.creds.SepayAPIKey)
	}
	if len(validKeys) == 0 {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 401, Message: "VietQR webhooks not configured"}
	}
	matched := false
	for _, k := range validKeys {
		if strategy.TimingSafeStrEqual(k, provided) {
			matched = true
		}
	}
	if !matched {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 401, Message: "Invalid webhook authorization"}
	}

	var body struct {
		Data []struct {
			Description string  `json:"description"`
			Amount      float64 `json:"amount"`
		} `json:"data"`
		Content        string  `json:"content"`
		TransferAmount float64 `json:"transferAmount"`
		TransactionID  any     `json:"transactionId"`
		Description    string  `json:"description"`
		CreditAmount   float64 `json:"creditAmount"`
	}
	_ = json.Unmarshal(payload, &body)

	// Casso: data[] of statement lines.
	for _, tx := range body.Data {
		if ref := extractRef(tx.Description); ref != "" && tx.Amount > 0 {
			return strategy.WebhookAction{Type: strategy.ActionPaymentCompleted, ReferenceCode: ref}, nil
		}
	}
	// SePay: content + transferAmount.
	if body.Content != "" && body.TransferAmount > 0 {
		if ref := extractRef(body.Content); ref != "" {
			return strategy.WebhookAction{Type: strategy.ActionPaymentCompleted, ReferenceCode: ref}, nil
		}
	}
	// MB Bank BaaS: transactionId + description + creditAmount.
	if body.TransactionID != nil && body.Description != "" {
		if ref := extractRef(body.Description); ref != "" && body.CreditAmount > 0 {
			return strategy.WebhookAction{Type: strategy.ActionPaymentCompleted, ReferenceCode: ref}, nil
		}
	}
	return strategy.Ignored(), nil
}

// stripApikeyPrefix removes a leading case-insensitive "Apikey " (Node
// .replace(/^Apikey\s+/i, ”)).
func stripApikeyPrefix(h string) string {
	const p = "apikey"
	t := strings.TrimSpace(h)
	if len(t) >= len(p) && strings.EqualFold(t[:len(p)], p) {
		return strings.TrimLeft(t[len(p):], " \t")
	}
	return t
}
