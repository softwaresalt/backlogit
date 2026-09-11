package auditsuccess_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"testing"
)

// TestAnalyzer invokes the test-local analysistest contract for FL004. Keeping
// the production import below testdata lets this package compile before the
// brand-new analyzer declaration lands.
func TestAnalyzer(t *testing.T) {
	for _, path := range []string{"analyzer.go", "sinks.go"} {
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("FL004 analyzer is not implemented: %s is absent", path)
		} else if err != nil {
			t.Fatalf("inspect FL004 %s: %v", path, err)
		}
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
		t.Fatalf("FL004 analyzer contract failed: %v\n%s", err, output)
	}
}
