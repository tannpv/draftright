package shared

import "time"

// ISOMillis formats t as Node's Date.toISOString() does: UTC, exactly
// three fractional-second digits, trailing "Z". Matches the JSON that
// TypeORM emits for timestamp columns (created_at/updated_at/expires_at)
// on the Phase 2 entitlement endpoints.
func ISOMillis(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}
