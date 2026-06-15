package errreport

import (
	"context"
	"errors"
)

// ErrInvalidPlatform is returned (→ 400) for a platform outside the allowlist.
// The message is the exact Node BadRequestException text so the 400 body is
// byte-identical to NestJS.
var ErrInvalidPlatform = errors.New("platform must be one of: ios, android, macos, windows, linux, web")

// Repo is the consumer-side port.
type Repo interface {
	FindByFingerprint(ctx context.Context, fp string) (*Existing, error)
	Insert(ctx context.Context, n NewRow) (*Existing, error)
	BumpDedup(ctx context.Context, fp string, appVersion, userID, deviceID *string, context []byte) (*Existing, error)
}

// Service ingests error reports.
type Service struct{ repo Repo }

// NewService wires the repo.
func NewService(repo Repo) *Service { return &Service{repo: repo} }

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Ingest ports ErrorsService.ingest: validate platform, coerce severity,
// slice, fingerprint, dedup-or-insert. userID is the best-effort JWT subject
// ("" when anonymous).
func (s *Service) Ingest(ctx context.Context, in CreateErrorReport, userID string) (*Existing, error) {
	if !PlatformValid(in.Platform) {
		return nil, ErrInvalidPlatform
	}
	severity := CoerceSeverity(in.Severity)
	message := sliceUTF16(in.Message, 5000)
	stack := sliceUTF16(in.StackTrace, 20000)
	errType := sliceUTF16(in.ErrorType, 200)
	fp := Fingerprint(errType, stack)

	existing, err := s.repo.FindByFingerprint(ctx, fp)
	if err != nil {
		return nil, err
	}
	var uid *string
	if userID != "" {
		uid = &userID
	}
	devID := ptrOrNil(sliceUTF16(in.DeviceID, 100))
	var ctxBytes []byte
	if len(in.Context) > 0 {
		ctxBytes = in.Context
	}

	if existing != nil {
		res, err := s.repo.BumpDedup(ctx, fp, ptrOrNil(in.AppVersion), uid, devID, ctxBytes)
		if err != nil {
			return nil, err
		}
		res.Fingerprint = fp
		return res, nil
	}
	// New row: scrub message + stack (→ nil when empty after scrub).
	scrubbedMsg := Scrub(message)
	scrubbedStack := Scrub(stack)
	res, err := s.repo.Insert(ctx, NewRow{
		Platform:    in.Platform,
		AppVersion:  ptrOrNil(in.AppVersion),
		Severity:    severity,
		ErrorType:   ptrOrNil(errType),
		Message:     ptrOrNil(scrubbedMsg),
		StackTrace:  ptrOrNil(scrubbedStack),
		Context:     ctxBytes,
		UserID:      uid,
		DeviceID:    devID,
		Fingerprint: fp,
	})
	if err != nil {
		return nil, err
	}
	res.Fingerprint = fp
	return res, nil
}
