package bugreports

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestSaveWritesPngUnderDatedDir(t *testing.T) {
	root := t.TempDir()
	day := time.Date(2026, 6, 15, 9, 30, 0, 0, time.UTC)
	s := NewStorage(root, fixedClock(day))

	buf := []byte("\x89PNG fake bytes")
	path, filename, err := s.Save(buf, "image/png", "shot.png")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	dir := filepath.Join(root, "2026-06-15")
	if filepath.Dir(path) != dir {
		t.Fatalf("path dir = %q, want %q", filepath.Dir(path), dir)
	}
	if filepath.Ext(path) != ".png" {
		t.Fatalf("ext = %q, want .png", filepath.Ext(path))
	}
	// Node persists screenshot_filename = file.originalname (when present).
	if filename != "shot.png" {
		t.Fatalf("filename = %q, want shot.png", filename)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != string(buf) {
		t.Fatalf("written bytes = %q, want %q", got, buf)
	}
}

func TestSaveJpegExtension(t *testing.T) {
	s := NewStorage(t.TempDir(), fixedClock(time.Now()))
	path, _, err := s.Save([]byte("jpg"), "image/jpeg", "p.jpg")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if filepath.Ext(path) != ".jpg" {
		t.Fatalf("ext = %q, want .jpg", filepath.Ext(path))
	}
}

func TestSaveFallsBackToStoredNameWhenNoOriginal(t *testing.T) {
	s := NewStorage(t.TempDir(), fixedClock(time.Now()))
	path, filename, err := s.Save([]byte("x"), "image/png", "")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Node: screenshot_filename = file.originalname || filename (= <uuid>.ext).
	if filename != filepath.Base(path) {
		t.Fatalf("filename = %q, want stored name %q", filename, filepath.Base(path))
	}
}

func TestSaveRejectsUnsupportedMime(t *testing.T) {
	s := NewStorage(t.TempDir(), fixedClock(time.Now()))
	_, _, err := s.Save([]byte("x"), "image/webp", "p.webp")
	if err == nil {
		t.Fatal("expected error for image/webp, got nil")
	}
	if err.Error() != "only PNG or JPEG screenshots are accepted" {
		t.Fatalf("error = %q, want Node string", err.Error())
	}
}
