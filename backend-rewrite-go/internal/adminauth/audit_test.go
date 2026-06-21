package adminauth

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAdminUserAuditOut_MarshalJSON_FieldOrderAndTimestamp(t *testing.T) {
	row := AdminUserAuditOut{
		ID:            "11111111-1111-1111-1111-111111111111",
		ActorAdminID:  "22222222-2222-2222-2222-222222222222",
		ActorEmail:    "admin@draftright.info",
		TargetAdminID: "33333333-3333-3333-3333-333333333333",
		TargetEmail:   "ops@draftright.info",
		CreatedAt:     time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"id":"11111111-1111-1111-1111-111111111111","actor_admin_id":"22222222-2222-2222-2222-222222222222","actor_email":"admin@draftright.info","target_admin_id":"33333333-3333-3333-3333-333333333333","target_email":"ops@draftright.info","created_at":"2026-06-21T12:00:00.000Z"}`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
}
