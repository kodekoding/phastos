package benchmark

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func TestMain(m *testing.M) {
	// Suppress all zerolog output so Phastos' requestLogger middleware
	// doesn't corrupt Go benchmark result lines on stdout.
	// The logging code still executes (preserving accurate benchmark timing),
	// but the actual write is short-circuited at the global level check.
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Exit(m.Run())
}
