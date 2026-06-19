package updates

import (
	"context"
	"testing"
)

type fakeRepo struct {
	preferred map[string]string
	releases  map[string]*Release // key "platform/channel"
}

func (f *fakeRepo) PreferredChannel(_ context.Context, p string) (string, error) {
	if c, ok := f.preferred[p]; ok {
		return c, nil
	}
	return "direct", nil
}
func (f *fakeRepo) EnabledRelease(_ context.Context, p, c string) (*Release, error) {
	return f.releases[p+"/"+c], nil
}

func TestGetEffective_FallbackToOtherChannel(t *testing.T) {
	r := &fakeRepo{
		preferred: map[string]string{"mac": "store"},
		releases:  map[string]*Release{"mac/direct": {Platform: "mac", Version: "2.2.9", Channel: "direct"}},
	}
	s := NewService(r)
	got, _ := s.getEffective(context.Background(), "mac", "")
	if got == nil || got.Channel != "direct" {
		t.Fatalf("expected fallback to direct, got %+v", got)
	}
}

func TestGetEffective_InvalidOverrideIgnored(t *testing.T) {
	r := &fakeRepo{
		preferred: map[string]string{"mac": "direct"},
		releases:  map[string]*Release{"mac/direct": {Platform: "mac", Version: "2.2.9", Channel: "direct"}},
	}
	s := NewService(r)
	got, _ := s.getEffective(context.Background(), "mac", "garbage") // ignored → policy default
	if got == nil || got.Version != "2.2.9" {
		t.Fatalf("garbage override should be ignored, got %+v", got)
	}
}
