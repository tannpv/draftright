package inbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSvc struct {
	counts    Counts
	feed      Feed
	lastQuery FeedQuery
}

func (f *fakeSvc) Counts(context.Context) (Counts, error) { return f.counts, nil }
func (f *fakeSvc) FeedFor(_ context.Context, q FeedQuery) (Feed, error) {
	f.lastQuery = q
	return f.feed, nil
}

func newH(s handlerService) *Handler { return &Handler{svc: s} }

func TestCountsHandler(t *testing.T) {
	svc := &fakeSvc{counts: Counts{NewBugs: 1, NewFeatures: 2, NewErrors: 3, Total: 6}}
	req := httptest.NewRequest("GET", "/admin/inbox/counts", nil)
	rec := httptest.NewRecorder()
	newH(svc).Counts(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var got Counts
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got != svc.counts {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestFeedHandler_ParsesQuery(t *testing.T) {
	svc := &fakeSvc{feed: Feed{Items: []any{}, Counts: FeedCounts{}}}
	req := httptest.NewRequest("GET", "/admin/inbox?kind=bug&status=open&limit=25", nil)
	rec := httptest.NewRecorder()
	newH(svc).Feed(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if svc.lastQuery.Kind != "bug" || !svc.lastQuery.OpenOnly || svc.lastQuery.Limit != 25 {
		t.Fatalf("query parse wrong: %+v", svc.lastQuery)
	}
}

func TestInboxLimit(t *testing.T) {
	cases := map[string]int{
		"":     50,  // empty → '50'
		"abc":  50,  // NaN → 50
		"0":    50,  // 0 falsy → 50
		"25":   25,  // pass-through
		"200":  100, // cap
		"-5":   -5,  // negative passes (Node: -5 || 50 = -5, min(-5,100)=-5)
		"12ab": 12,  // JS parseInt stops at non-digit
	}
	for in, want := range cases {
		if got := inboxLimit(in); got != want {
			t.Fatalf("inboxLimit(%q)=%d want %d", in, got, want)
		}
	}
}

func TestFeedHandler_EmptyItemsRendersArray(t *testing.T) {
	svc := &fakeSvc{feed: Feed{Items: []any{}, Counts: FeedCounts{Errors: 0, Bugs: 0, Returned: 0}}}
	req := httptest.NewRequest("GET", "/admin/inbox", nil)
	rec := httptest.NewRecorder()
	newH(svc).Feed(rec, req)
	if !contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("empty items should render []: %s", rec.Body.String())
	}
	// default limit 50 when absent
	if svc.lastQuery.Limit != 50 {
		t.Fatalf("default limit should be 50, got %d", svc.lastQuery.Limit)
	}
}
