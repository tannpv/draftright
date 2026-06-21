package provenance_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/provenance"
)

func TestNewContext_AttachesReadableCarrier(t *testing.T) {
	ctx, p := provenance.NewContext(context.Background())
	require.NotNil(t, p)
	require.Same(t, p, provenance.From(ctx))
	m, ty := p.Read()
	require.Equal(t, "", m)
	require.Equal(t, "", ty)
}

func TestFrom_NoCarrier_ReturnsNil(t *testing.T) {
	require.Nil(t, provenance.From(context.Background()))
}

func TestStamp_LastWriteWins(t *testing.T) {
	ctx, p := provenance.NewContext(context.Background())
	provenance.Stamp(ctx, "gpt-4o-mini", "openai")
	provenance.Stamp(ctx, "claude-3-5-sonnet-20241022", "anthropic")
	m, ty := p.Read()
	require.Equal(t, "claude-3-5-sonnet-20241022", m)
	require.Equal(t, "anthropic", ty)
}

func TestPackageStamp_NoCarrier_NoPanic(t *testing.T) {
	require.NotPanics(t, func() {
		provenance.Stamp(context.Background(), "m", "t")
	})
}

func TestStamp_RaceClean(t *testing.T) {
	ctx, p := provenance.NewContext(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); provenance.Stamp(ctx, "m", "t") }()
	}
	wg.Wait()
	m, ty := p.Read()
	require.Equal(t, "m", m)
	require.Equal(t, "t", ty)
}
