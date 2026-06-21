package adminauth

import (
	"encoding/json"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// AdminUserAuditOut is one admin_user_audit_log row as GET /admin/admin-user-audit
// serializes it: snake_case, fixed key order, created_at as ISO-8601 millis (UTC)
// — matching the timestamp format every other admin response uses (shared.ISOMillis).
type AdminUserAuditOut struct {
	ID            string
	ActorAdminID  string
	ActorEmail    string
	TargetAdminID string
	TargetEmail   string
	CreatedAt     time.Time
}

func (a AdminUserAuditOut) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            string `json:"id"`
		ActorAdminID  string `json:"actor_admin_id"`
		ActorEmail    string `json:"actor_email"`
		TargetAdminID string `json:"target_admin_id"`
		TargetEmail   string `json:"target_email"`
		CreatedAt     string `json:"created_at"`
	}{
		ID:            a.ID,
		ActorAdminID:  a.ActorAdminID,
		ActorEmail:    a.ActorEmail,
		TargetAdminID: a.TargetAdminID,
		TargetEmail:   a.TargetEmail,
		CreatedAt:     shared.ISOMillis(a.CreatedAt),
	})
}
