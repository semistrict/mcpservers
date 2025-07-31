package metabasemcp

import (
	"os"
	"testing"
)

// skipIfNoMetabase skips the test if Metabase integration tests are not enabled.
// Set RUN_METABASE_TESTS=1 to run these tests.
func skipIfNoMetabase(t *testing.T) {
	t.Helper()
	if os.Getenv("RUN_METABASE_TESTS") != "1" {
		t.Skip("Skipping Metabase integration test. Set RUN_METABASE_TESTS=1 to run.")
	}
}