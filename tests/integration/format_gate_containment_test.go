package integration_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var unscopedGofmtLine = regexp.MustCompile(`\bgofmt\s+-l\s+\.(?:$|\s|[;|&)"'])`)

func TestU1_MakefileFormatGateScopesSourceDirectoriesAndCoversTrackedGo(t *testing.T) {
	repoRoot := testRepoRoot(t)

	t.Run("Makefile fmt target", func(t *testing.T) {
		content := readGovernedSurface(t, repoRoot, "Makefile")
		recipe := localMakeRecipeLines(content, "fmt")
		require.NotEmpty(t, recipe, "Makefile fmt target must have at least one recipe line")

		var hasScopedGofmt bool
		for _, line := range recipe {
			if strings.Contains(line, "gofmt -l cmd internal tests scripts") {
				hasScopedGofmt = true
				break
			}
		}
		assert.True(t, hasScopedGofmt, "Makefile fmt recipe must invoke `gofmt -l cmd internal tests scripts`")

		for i, line := range strings.Split(content, "\n") {
			assert.False(t, unscopedGofmtLine.MatchString(line), "Makefile line %d must not contain unscoped `gofmt -l .`: %s", i+1, line)
		}
	})

	t.Run("Go source directory completeness", func(t *testing.T) {
		var goFiles []string
		_, statErr := os.Stat(filepath.Join(repoRoot, ".git"))
		switch {
		case statErr == nil:
			cmd := exec.CommandContext(t.Context(), "git", "-C", repoRoot, "ls-files", "-z", "--", "*.go")
			out, err := cmd.Output()
			require.NoError(t, err, "git ls-files for *.go must succeed")
			for _, raw := range bytes.Split(out, []byte{0}) {
				if len(raw) == 0 {
					continue
				}
				goFiles = append(goFiles, filepath.ToSlash(string(raw)))
			}
		case errors.Is(statErr, os.ErrNotExist):
			err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() {
					if d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(d.Name(), ".go") {
					rel, relErr := filepath.Rel(repoRoot, p)
					if relErr != nil {
						return relErr
					}
					goFiles = append(goFiles, filepath.ToSlash(rel))
				}
				return nil
			})
			require.NoError(t, err, "walking repoRoot for .go files must succeed")
		default:
			require.NoError(t, statErr)
		}

		require.NotEmpty(t, goFiles, "repository must contain tracked .go files")

		allowedDirs := map[string]bool{
			"cmd":      true,
			"internal": true,
			"tests":    true,
			"scripts":  true,
		}

		for _, rel := range goFiles {
			assert.NotEqual(t, ".", path.Dir(rel), "root-level Go file %q is not allowed", rel)
			top := strings.SplitN(rel, "/", 2)[0]
			assert.True(t, allowedDirs[top], "Go file %q resides in top-level directory %q outside {cmd, internal, tests, scripts}", rel, top)
		}
	})
}
