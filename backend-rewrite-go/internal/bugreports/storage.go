package bugreports

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// errUnsupportedMime mirrors the Node service's BadRequestException string
// (bug-reports.service.ts extensionFor). Byte-for-byte for the shadow gate.
var errUnsupportedMime = errors.New("only PNG or JPEG screenshots are accepted")

// Storage writes bug-report screenshots to disk under a dated directory,
// mirroring BugReportsService's create() file-handling. root is the
// BUG_REPORTS_DIR (bind-mounted in prod). now is injectable for tests.
type Storage struct {
	root string
	now  func() time.Time
}

// NewStorage wires the storage root and clock. Pass time.Now in production.
func NewStorage(root string, now func() time.Time) *Storage {
	return &Storage{root: root, now: now}
}

// extensionFor maps a mimetype to a file extension, mirroring Node:
// image/png → .png, image/jpeg|image/jpg → .jpg, else error.
func extensionFor(mimetype string) (string, error) {
	switch mimetype {
	case "image/png":
		return ".png", nil
	case "image/jpeg", "image/jpg":
		return ".jpg", nil
	default:
		return "", errUnsupportedMime
	}
}

// Save writes buf to <root>/YYYY-MM-DD/<uuid><ext> and returns the full
// path plus the screenshot_filename to persist. filename = originalName
// when non-empty, else the stored <uuid><ext> name (Node: originalname ||
// filename).
func (s *Storage) Save(buf []byte, mimetype, originalName string) (path, filename string, err error) {
	ext, err := extensionFor(mimetype)
	if err != nil {
		return "", "", err
	}
	day := s.now().Format("2006-01-02")
	dir := filepath.Join(s.root, day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	name := uuid.NewString() + ext
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, buf, 0o644); err != nil {
		return "", "", err
	}
	filename = originalName
	if filename == "" {
		filename = name
	}
	return full, filename, nil
}
