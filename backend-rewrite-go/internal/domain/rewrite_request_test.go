package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/domain"
)

func TestNewRewriteRequest_HappyPath(t *testing.T) {
	r, err := domain.NewRewriteRequest("Hello world.", "polished", "")
	require.NoError(t, err)
	require.Equal(t, "Hello world.", r.Text())
	require.Equal(t, domain.TonePolished, r.Tone())
	require.Equal(t, int32(12), r.InputLength())
}

func TestNewRewriteRequest_TrimsWhitespace(t *testing.T) {
	r, err := domain.NewRewriteRequest("   hi   ", "natural", "")
	require.NoError(t, err)
	require.Equal(t, "hi", r.Text())
}

func TestNewRewriteRequest_RejectsEmpty(t *testing.T) {
	_, err := domain.NewRewriteRequest("   ", "polished", "")
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestNewRewriteRequest_RejectsTooLong(t *testing.T) {
	tooLong := strings.Repeat("a", domain.MaxInputChars+1)
	_, err := domain.NewRewriteRequest(tooLong, "polished", "")
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestNewRewriteRequest_RejectsUnknownTone(t *testing.T) {
	_, err := domain.NewRewriteRequest("hi", "shouty", "")
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestNewRewriteRequest_TranslateRequiresLang(t *testing.T) {
	_, err := domain.NewRewriteRequest("hello", "translate", "")
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestNewRewriteRequest_TranslateAcceptsLang(t *testing.T) {
	r, err := domain.NewRewriteRequest("hello", "translate", "vi")
	require.NoError(t, err)
	require.Equal(t, "vi", r.Lang())
}

func TestParseTone_AcceptsAllSupported(t *testing.T) {
	supported := []domain.Tone{
		domain.ToneSimple, domain.ToneNatural, domain.TonePolished,
		domain.ToneConcise, domain.ToneTechnical, domain.ToneClaude,
		domain.ToneTranslate,
	}
	for _, want := range supported {
		got, err := domain.ParseTone(string(want))
		require.NoError(t, err, "tone %q", want)
		require.Equal(t, want, got)
	}
}

func TestParseTone_RejectsUnknown(t *testing.T) {
	_, err := domain.ParseTone("frenzied")
	require.True(t, errors.Is(err, domain.ErrInvalidInput))
}
