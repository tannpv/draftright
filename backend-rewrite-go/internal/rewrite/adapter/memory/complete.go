package memory

import "context"

// SetCompletion scripts the text returned by Complete.
func (p *Provider) SetCompletion(text string) { p.completion = text }

// Complete returns the scripted completion (blocking-provider fake).
// Satisfies the aicall.Completer port for tests that exercise the
// blocking extraction path without a real upstream.
func (p *Provider) Complete(_ context.Context, _, _ string) (string, int64, error) {
	return p.completion, 1, nil
}
