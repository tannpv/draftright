package bugreports

import "testing"

// toUUID is pure logic (no DB). ResolveUserID/Insert are DB-bound and, like
// errreport, are left to the live shadow gate rather than a fake-DB unit test.

func TestToUUIDValid(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	u := toUUID(&id)
	if !u.Valid {
		t.Fatal("expected Valid uuid")
	}
}

func TestToUUIDNilOrEmptyOrBad(t *testing.T) {
	empty := ""
	bad := "not-a-uuid"
	for _, in := range []*string{nil, &empty, &bad} {
		if u := toUUID(in); u.Valid {
			t.Fatalf("expected zero/invalid uuid for %v", in)
		}
	}
}
