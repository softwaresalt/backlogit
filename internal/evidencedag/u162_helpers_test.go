package evidencedag

import (
	"os"
	"testing"
)

var testForbiddenRoot string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "evidencedag-forbidden-")
	if err != nil {
		panic(err)
	}
	testForbiddenRoot = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// openLedger opens a ledger with an unrelated forbidden root, as a caller with
// a real evidence root would.
func openLedger(path string) (*Ledger, error) { return OpenLedger(path, testForbiddenRoot) }
