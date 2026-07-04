package domain

import (
	"errors"
	"testing"
)

func TestParseInputKind(t *testing.T) {
	if k, err := ParseInputKind(""); err != nil || k != InputKindTyped {
		t.Fatalf("empty must default typed, got %v %v", k, err)
	}
	if k, err := ParseInputKind("speech"); err != nil || k != InputKindSpeech {
		t.Fatalf("speech: %v %v", k, err)
	}
	if _, err := ParseInputKind("banana"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("banana must be ErrInvalidInput")
	}
}
