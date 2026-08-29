package payment

import (
	"context"
	"fmt"
	"time"
)

// RedeemAppleTransaction verifies a client-supplied StoreKit transaction, grants
// Pro through the same Grant chokepoint every other provider uses (cancelling
// any prior active sub), stamps the ORIGINAL transaction id so later App Store
// Server Notifications (renew/expire/refund) match this row, and sends the
// activation email — parity with the other providers' first grant
// (activateSubscription, webhook.go:244). No second grant path.
func (s *Service) RedeemAppleTransaction(ctx context.Context, userID, signedTransaction string) error {
	tx, err := s.appleVerify(signedTransaction) // seam over applestore.Verifier.Verify
	if err != nil {
		return fmt.Errorf("apple verify: %w", err)
	}
	planID, billing, err := s.resolvePlanIDFromAppleProduct(ctx, tx.ProductID)
	if err != nil {
		return err
	}
	exp := time.UnixMilli(tx.ExpiresDate).UTC()
	if tx.ExpiresDate == 0 { // transaction omitted expiresDate: fall back to billing period
		now := s.now()
		if billing == "yearly" {
			exp = now.AddDate(1, 0, 0)
		} else {
			exp = now.AddDate(0, 1, 0)
		}
	}
	if err := s.subsWriter.Grant(ctx, userID, planID, string(StoreAppleIAP), &exp); err != nil {
		return err
	}
	// Stamp the ORIGINAL transaction id onto the just-granted row, matched by
	// user_id + store_type — NOT by a payments reference, which doesn't exist
	// for IAP (see StampStoreRefByUser) — so renewals/expiries/refunds match
	// via Extend/CancelByStoreRef/ExpireByStoreRef.
	if err := s.subsWriter.StampStoreRefByUser(ctx, userID, string(StoreAppleIAP), tx.OriginalTransactionID); err != nil {
		return err
	}
	// First IAP purchase emails like every other provider (webhook.go:256-258).
	if email, name, err := s.webhookRepo.UserEmailName(ctx, userID); err == nil && email != "" {
		s.emailer.SubscriptionActivated(ctx, email, name, "Pro")
	}
	return nil
}
