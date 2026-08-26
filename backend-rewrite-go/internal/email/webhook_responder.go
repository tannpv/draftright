package email

import (
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/emailkit"
)

// nonRetryableWebhookErrs are the sentinels emailkit documents as NOT
// retryable: resending the identical request cannot succeed. See
// classifyWebhookErr for why everything else defaults the other way.
var nonRetryableWebhookErrs = []error{
	emailkit.ErrBadSignature,
	emailkit.ErrStale,
	emailkit.ErrBadPayload,
}

// classifyWebhookErr maps an error from emailkit.WebhookHandler.Handle to
// this project's error code and whether the failure is retryable.
// Retryable errors answer 5xx so the provider redelivers; non-retryable ones
// answer 4xx.
//
// Deliberately inverted from "ErrStoreFailure => 5xx, else 4xx": that shape
// answers a sentinel this package doesn't recognise yet with a 4xx, which
// tells the provider not to redeliver and drops the event silently. Matching
// the known non-retryable set explicitly and defaulting everything else
// (ErrStoreFailure today, whatever emailkit adds tomorrow) to retryable means
// an unrecognised failure gets retried — and logged — instead of discarded.
func classifyWebhookErr(err error) (code shared.ErrorCode, retryable bool) {
	for _, sentinel := range nonRetryableWebhookErrs {
		if errors.Is(err, sentinel) {
			return shared.CodeInvalidInput, false
		}
	}
	return shared.CodeInternal, true
}

// WebhookResponder adapts emailkit's WebhookHandler.Handle to this project's
// http.HandlerFunc + error-envelope contract. Extracted out of main.go's
// composeDeps closure into a named, exported function so the 500-vs-400
// mapping is unit-testable without standing up the whole server — main.go's
// wiring is now a single line.
//
// The HTTP response body is the SAME opaque message for every non-retryable
// sentinel: telling a caller which check failed tells an attacker which half
// of the request to fix. The one place that distinction survives is the log
// line below — emailkit keeps ErrBadSignature and ErrStale as separate
// sentinels precisely so an operator reading server logs can tell a rotated
// webhook secret from clock skew, and it wraps the underlying store error
// into ErrStoreFailure precisely so the actual cause of a 500 isn't lost.
// Discarding the error before logging it — as this mount used to — leaves no
// server-side record of what failed at all.
func WebhookResponder(h *emailkit.WebhookHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h.Handle(w, r)
		if err == nil {
			return // Handle wrote the success body itself
		}

		log := shared.LogFromContext(r.Context())
		code, retryable := classifyWebhookErr(err)
		if retryable {
			// A store failure (or a sentinel this package doesn't recognise
			// yet) is an operational problem, not routine internet noise.
			log.Error("emailkit webhook: answering 5xx for redelivery", "error", err)
			shared.WriteError(w, r, code, "Webhook processing failed")
			return
		}
		// Bad signature / stale timestamp / malformed payload are routine
		// internet noise (scanners, a rotated secret, clock skew). Logged at
		// warn, not error — and it's the sentinel in "error", never the HTTP
		// response, that lets an operator tell them apart.
		log.Warn("emailkit webhook: rejecting request", "error", err)
		shared.WriteError(w, r, code, "Invalid webhook signature")
	}
}
