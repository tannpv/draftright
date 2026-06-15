package updates

import "context"

// Repo is the read port (consumer-side interface).
type Repo interface {
	PreferredChannel(ctx context.Context, platform string) (string, error)
	EnabledRelease(ctx context.Context, platform, channel string) (*Release, error)
}

// Service resolves effective releases. Ports ReleasesService read paths.
type Service struct{ repo Repo }

// NewService wires the repo.
func NewService(repo Repo) *Service { return &Service{repo: repo} }

func validChannel(c string) bool { return c == "direct" || c == "store" }

func otherChannel(c string) string {
	if c == "direct" {
		return "store"
	}
	return "direct"
}

// getEffective ports ReleasesService.getEffective: desired channel =
// valid override else policy.preferred; try enabled release there, else
// fall back to the other channel, else nil.
func (s *Service) getEffective(ctx context.Context, platform, override string) (*Release, error) {
	desired := override
	if !validChannel(desired) {
		pref, err := s.repo.PreferredChannel(ctx, platform)
		if err != nil {
			return nil, err
		}
		desired = pref
	}
	rel, err := s.repo.EnabledRelease(ctx, platform, desired)
	if err != nil {
		return nil, err
	}
	if rel != nil {
		return rel, nil
	}
	return s.repo.EnabledRelease(ctx, platform, otherChannel(desired))
}

// listEffective resolves every platform in fixed order.
func (s *Service) listEffective(ctx context.Context, override string) (map[string]*Release, error) {
	out := make(map[string]*Release, len(Platforms))
	for _, p := range Platforms {
		rel, err := s.getEffective(ctx, p, override)
		if err != nil {
			return nil, err
		}
		out[p] = rel
	}
	return out, nil
}
