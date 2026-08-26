package email

import (
	"testing"

	"github.com/tannpv/emailkit"
)

// PgRepo is handed straight to emailkit as the Store — no adapter type. This
// fails to compile the moment a method name or signature drifts, which is the
// point: the drift would otherwise surface as a runtime wiring error in main.
func TestPgRepoSatisfiesEmailkitStore(t *testing.T) {
	var _ emailkit.Store = (*PgRepo)(nil)
}
