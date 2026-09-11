package errwrap_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"testing"
)

// TestAnalyzer invokes the test-local analysistest contract for FL002. Keeping
// the production import below testdata lets this package compile before the
// brand-new analyzer declaration lands.
func TestAnalyzer(t *testing.T) {
	if _, err := os.Stat("analyzer.go"); errors.Is(err, fs.ErrNotExist) {
		t.Fatal("FL002 analyzer is not implemented: analyzer.go is absent")
	} else if err != nil {
		t.Fatalf("inspect FL002 analyzer.go: %v", err)
	}

	command := exec.Command(
		"go",
		"test",
		"-count=1",
		"-run",
		"^TestContractAnalyzer$",
		"./testdata/harness",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("FL002 analyzer contract failed: %v\n%s", err, output)
	}
}
