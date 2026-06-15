package errreport

import (
	"context"
	"testing"
)

type fakeRepo struct {
	existing *Existing
	inserted *NewRow
	bumped   bool
}

func (f *fakeRepo) FindByFingerprint(context.Context, string) (*Existing, error) {
	return f.existing, nil
}

func (f *fakeRepo) Insert(_ context.Context, n NewRow) (*Existing, error) {
	f.inserted = &n
	return &Existing{ID: "new-id", DisplayNo: 7, Count: 1}, nil
}

func (f *fakeRepo) BumpDedup(context.Context, string, *string, *string, *string, []byte) (*Existing, error) {
	f.bumped = true
	return &Existing{ID: "old-id", DisplayNo: 3, Count: 5}, nil
}

func TestIngest_NewRowScrubsAndInserts(t *testing.T) {
	f := &fakeRepo{existing: nil}
	s := NewService(f)
	res, err := s.Ingest(context.Background(), CreateErrorReport{
		Platform: "ios", Message: "token Bearer abc.def leaked", StackTrace: "at f1\nat f2",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if f.inserted == nil {
		t.Fatal("expected insert")
	}
	if f.inserted.Message == nil || *f.inserted.Message == "token Bearer abc.def leaked" {
		t.Fatalf("message not scrubbed: %v", f.inserted.Message)
	}
	if res.Count != 1 {
		t.Fatalf("count = %d, want 1", res.Count)
	}
	if len(res.Fingerprint) != 64 {
		t.Fatalf("fingerprint must be stamped on the result: %q", res.Fingerprint)
	}
}

func TestIngest_DedupHitBumps(t *testing.T) {
	f := &fakeRepo{existing: &Existing{ID: "old-id", DisplayNo: 3, Count: 4}}
	s := NewService(f)
	res, _ := s.Ingest(context.Background(), CreateErrorReport{Platform: "ios", StackTrace: "at f1"}, "")
	if f.inserted != nil {
		t.Fatal("dedup hit must NOT insert")
	}
	if !f.bumped || res.Count != 5 {
		t.Fatalf("expected bump, count=%d", res.Count)
	}
	if len(res.Fingerprint) != 64 {
		t.Fatalf("fingerprint must be stamped on dedup result: %q", res.Fingerprint)
	}
}

func TestIngest_InvalidPlatform400(t *testing.T) {
	s := NewService(&fakeRepo{})
	_, err := s.Ingest(context.Background(), CreateErrorReport{Platform: "symbian"}, "")
	if err == nil {
		t.Fatal("expected error for bad platform")
	}
	if err != ErrInvalidPlatform {
		t.Fatalf("err = %v, want ErrInvalidPlatform", err)
	}
}
