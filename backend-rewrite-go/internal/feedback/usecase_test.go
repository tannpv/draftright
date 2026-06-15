package feedback

import (
	"context"
	"errors"
	"testing"
)

// fakeRepo is an in-memory Repo for the no-DB use-case tests.
type fakeRepo struct {
	// feature ids that exist as kind='feature'
	features map[string]bool
	// set of (featureID|userID) votes
	votes map[string]bool
	// per-feature persisted vote_count (last UpdateVoteCount)
	counts map[string]int

	inserted        *NewRow
	insertedKind    string
	resolveOverride func(id *string) (*string, error)

	listRows  []FeatureRow
	listTotal int64
	votedIDs  map[string]bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		features: map[string]bool{},
		votes:    map[string]bool{},
		counts:   map[string]int{},
		votedIDs: map[string]bool{},
	}
}

func voteKey(featureID, userID string) string { return featureID + "|" + userID }

func (f *fakeRepo) ResolveUserID(ctx context.Context, id *string) (*string, error) {
	if f.resolveOverride != nil {
		return f.resolveOverride(id)
	}
	return id, nil
}

func (f *fakeRepo) Insert(ctx context.Context, n NewRow) (Created, error) {
	cp := n
	f.inserted = &cp
	f.insertedKind = n.Kind
	return Created{ID: "new-id", DisplayNo: 7, Kind: n.Kind}, nil
}

func (f *fakeRepo) FeatureExists(ctx context.Context, featureID string) (bool, error) {
	return f.features[featureID], nil
}

func (f *fakeRepo) VoteExists(ctx context.Context, featureID, userID string) (bool, error) {
	return f.votes[voteKey(featureID, userID)], nil
}

func (f *fakeRepo) InsertVote(ctx context.Context, featureID, userID string) error {
	f.votes[voteKey(featureID, userID)] = true
	return nil
}

func (f *fakeRepo) DeleteVote(ctx context.Context, featureID, userID string) error {
	delete(f.votes, voteKey(featureID, userID))
	return nil
}

func (f *fakeRepo) CountVotes(ctx context.Context, featureID string) (int, error) {
	n := 0
	for k := range f.votes {
		if len(k) >= len(featureID)+1 && k[:len(featureID)+1] == featureID+"|" {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) UpdateVoteCount(ctx context.Context, featureID string, count int) error {
	f.counts[featureID] = count
	return nil
}

func (f *fakeRepo) CountFeatures(ctx context.Context, status, platform *string) (int64, error) {
	return f.listTotal, nil
}

func (f *fakeRepo) ListFeatures(ctx context.Context, status, platform *string, limit, offset int) ([]FeatureRow, error) {
	return f.listRows, nil
}

func (f *fakeRepo) VotedFeatureIDs(ctx context.Context, ids []string, userID string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, id := range ids {
		if f.votedIDs[id] {
			out[id] = true
		}
	}
	return out, nil
}

func baseFeatureInput() CreateInput {
	return CreateInput{
		Kind:        "feature",
		Title:       "Add dark mode",
		Platform:    "mobile",
		Description: "please",
		Source:      "web",
	}
}

// (a) feature with missing/empty title → ErrTitleRequired.
func TestCreateFeedback_FeatureMissingTitle(t *testing.T) {
	svc := NewService(newFakeRepo())
	in := baseFeatureInput()
	in.Title = "   " // whitespace → trims to empty
	_, err := svc.CreateFeedback(context.Background(), in, "")
	if !errors.Is(err, ErrTitleRequired) {
		t.Fatalf("want ErrTitleRequired, got %v", err)
	}
	if got := ErrTitleRequired.Error(); got != "title is required for a feature request (1-80 characters)" {
		t.Fatalf("title message drift: %q", got)
	}
}

func TestCreateFeedback_FeatureTitleTooLong(t *testing.T) {
	svc := NewService(newFakeRepo())
	in := baseFeatureInput()
	long := ""
	for i := 0; i < 81; i++ {
		long += "a"
	}
	in.Title = long
	_, err := svc.CreateFeedback(context.Background(), in, "")
	if !errors.Is(err, ErrTitleRequired) {
		t.Fatalf("want ErrTitleRequired for 81-char title, got %v", err)
	}
}

// (b) feature with bad target_platform → ErrBadTargetPlatform.
func TestCreateFeedback_FeatureBadPlatform(t *testing.T) {
	svc := NewService(newFakeRepo())
	in := baseFeatureInput()
	in.Platform = "watch"
	_, err := svc.CreateFeedback(context.Background(), in, "")
	if !errors.Is(err, ErrBadTargetPlatform) {
		t.Fatalf("want ErrBadTargetPlatform, got %v", err)
	}
	if got := ErrBadTargetPlatform.Error(); got != "target_platform must be one of: playground, mobile, windows, mac, linux" {
		t.Fatalf("platform message drift: %q", got)
	}
}

func TestCreateFeedback_FeatureMissingPlatform(t *testing.T) {
	svc := NewService(newFakeRepo())
	in := baseFeatureInput()
	in.Platform = ""
	_, err := svc.CreateFeedback(context.Background(), in, "")
	if !errors.Is(err, ErrBadTargetPlatform) {
		t.Fatalf("want ErrBadTargetPlatform for empty platform, got %v", err)
	}
}

func TestCreateFeedback_FeatureHappy(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	uid := "user-1"
	out, err := svc.CreateFeedback(context.Background(), baseFeatureInput(), uid)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Kind != "feature" || out.ID != "new-id" || out.DisplayNo != 7 {
		t.Fatalf("bad Created: %+v", out)
	}
	if repo.inserted == nil {
		t.Fatal("expected an insert")
	}
	if repo.inserted.Kind != "feature" || repo.inserted.Title == nil || *repo.inserted.Title != "Add dark mode" {
		t.Fatalf("bad insert row: %+v", repo.inserted)
	}
	if repo.inserted.TargetPlatform == nil || *repo.inserted.TargetPlatform != "mobile" {
		t.Fatalf("bad target platform: %+v", repo.inserted.TargetPlatform)
	}
	if repo.inserted.UserID == nil || *repo.inserted.UserID != uid {
		t.Fatalf("bad user id: %+v", repo.inserted.UserID)
	}
}

// bug path: title + platform are forced null, no validation on them.
func TestCreateFeedback_BugForcesNullTitlePlatform(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	in := CreateInput{
		Kind:        "bug",
		Title:       "ignored",
		Platform:    "watch", // invalid but ignored for bugs
		Description: "broke",
		Source:      "mac",
	}
	out, err := svc.CreateFeedback(context.Background(), in, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Kind != "bug" {
		t.Fatalf("want bug kind, got %q", out.Kind)
	}
	if repo.inserted.Title != nil || repo.inserted.TargetPlatform != nil {
		t.Fatalf("bug must null title/platform, got title=%v platform=%v", repo.inserted.Title, repo.inserted.TargetPlatform)
	}
}

// missing description / source still guard (after DTO).
func TestCreateFeedback_RequiresDescriptionAndSource(t *testing.T) {
	svc := NewService(newFakeRepo())
	in := baseFeatureInput()
	in.Description = "  "
	if _, err := svc.CreateFeedback(context.Background(), in, ""); !errors.Is(err, ErrDescriptionRequired) {
		t.Fatalf("want ErrDescriptionRequired, got %v", err)
	}
	in = baseFeatureInput()
	in.Source = ""
	if _, err := svc.CreateFeedback(context.Background(), in, ""); !errors.Is(err, ErrSourceRequired) {
		t.Fatalf("want ErrSourceRequired, got %v", err)
	}
}

// (c) ToggleVote: no prior vote → insert + count up + hasVoted=true.
func TestToggleVote_AddsVote(t *testing.T) {
	repo := newFakeRepo()
	repo.features["feat-1"] = true
	svc := NewService(repo)
	res, err := svc.ToggleVote(context.Background(), "feat-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.HasVoted || res.VoteCount != 1 {
		t.Fatalf("want hasVoted=true count=1, got %+v", res)
	}
	if !repo.votes[voteKey("feat-1", "user-1")] {
		t.Fatal("vote not inserted")
	}
	if repo.counts["feat-1"] != 1 {
		t.Fatalf("persisted count not updated: %d", repo.counts["feat-1"])
	}
}

// (c) ToggleVote: prior vote → delete + count down + hasVoted=false.
func TestToggleVote_RemovesVote(t *testing.T) {
	repo := newFakeRepo()
	repo.features["feat-1"] = true
	repo.votes[voteKey("feat-1", "user-1")] = true
	svc := NewService(repo)
	res, err := svc.ToggleVote(context.Background(), "feat-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.HasVoted || res.VoteCount != 0 {
		t.Fatalf("want hasVoted=false count=0, got %+v", res)
	}
	if repo.votes[voteKey("feat-1", "user-1")] {
		t.Fatal("vote not deleted")
	}
}

// (d) ToggleVote on missing / non-feature id → ErrFeatureNotFound.
func TestToggleVote_NotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	_, err := svc.ToggleVote(context.Background(), "missing", "user-1")
	if !errors.Is(err, ErrFeatureNotFound) {
		t.Fatalf("want ErrFeatureNotFound, got %v", err)
	}
	if got := ErrFeatureNotFound.Error(); got != "feature request not found" {
		t.Fatalf("notfound message drift: %q", got)
	}
}

// ListPublicFeatures: clamps page/limit, attaches viewerHasVoted per row.
func TestListPublicFeatures_ViewerHasVoted(t *testing.T) {
	repo := newFakeRepo()
	repo.listTotal = 2
	repo.listRows = []FeatureRow{{ID: "a"}, {ID: "b"}}
	repo.votedIDs = map[string]bool{"b": true}
	svc := NewService(repo)
	res, err := svc.ListPublicFeatures(context.Background(), ListParams{}, "user-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Total != 2 || len(res.Rows) != 2 {
		t.Fatalf("bad list result: %+v", res)
	}
	if res.Rows[0].ViewerHasVoted {
		t.Fatal("row a should not be voted")
	}
	if !res.Rows[1].ViewerHasVoted {
		t.Fatal("row b should be voted")
	}
}

// Anonymous viewer (userID="") → viewerHasVoted always false, no VotedFeatureIDs call.
func TestListPublicFeatures_AnonymousAllFalse(t *testing.T) {
	repo := newFakeRepo()
	repo.listRows = []FeatureRow{{ID: "a"}}
	repo.votedIDs = map[string]bool{"a": true} // would be true if queried
	svc := NewService(repo)
	res, err := svc.ListPublicFeatures(context.Background(), ListParams{}, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Rows[0].ViewerHasVoted {
		t.Fatal("anonymous viewer must never have viewerHasVoted=true")
	}
}

// Clamping: page<1 → 1, limit>100 → 100, limit<1 → 1, default 20.
func TestListPublicFeatures_Clamping(t *testing.T) {
	repo := &clampSpyRepo{fakeRepo: newFakeRepo()}
	svc := NewService(repo)
	cases := []struct {
		page, limit           int
		wantLimit, wantOffset int
	}{
		{0, 0, 20, 0},      // defaults
		{-5, -5, 1, 0},     // page clamps to 1, limit clamps to 1
		{2, 200, 100, 100}, // limit clamps to 100, offset=(2-1)*100
		{3, 25, 25, 50},    // offset=(3-1)*25
	}
	for _, c := range cases {
		_, err := svc.ListPublicFeatures(context.Background(), ListParams{Page: c.page, Limit: c.limit}, "")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if repo.gotLimit != c.wantLimit || repo.gotOffset != c.wantOffset {
			t.Fatalf("page=%d limit=%d → got limit=%d offset=%d, want limit=%d offset=%d",
				c.page, c.limit, repo.gotLimit, repo.gotOffset, c.wantLimit, c.wantOffset)
		}
	}
}

type clampSpyRepo struct {
	*fakeRepo
	gotLimit, gotOffset int
}

func (s *clampSpyRepo) ListFeatures(ctx context.Context, status, platform *string, limit, offset int) ([]FeatureRow, error) {
	s.gotLimit = limit
	s.gotOffset = offset
	return s.fakeRepo.ListFeatures(ctx, status, platform, limit, offset)
}
