package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/config"
)

// TestCanonicalRestorePath asserts U1 (067.001-T): the resolver returns the
// repo-root-relative, .backlogit/-prefixed POSIX restore path derived purely
// from QueueLayout.RootDir, and rejects an absolute or parent-escaping RootDir
// by falling back to the default "queue" so the path stays workspace-contained.
func TestCanonicalRestorePath(t *testing.T) {
	tests := []struct {
		name     string
		rootDir  string
		basename string
		want     string
	}{
		{
			name:     "default queue layout",
			rootDir:  "",
			basename: "001-T.md",
			want:     ".backlogit/queue/001-T.md",
		},
		{
			name:     "configured root dir honored",
			rootDir:  "backlog",
			basename: "002-T.md",
			want:     ".backlogit/backlog/002-T.md",
		},
		{
			name:     "absolute root dir rejected (containment)",
			rootDir:  "/etc",
			basename: "003-T.md",
			want:     ".backlogit/queue/003-T.md",
		},
		{
			name:     "parent-escape root dir rejected (containment)",
			rootDir:  "../../evil",
			basename: "004-T.md",
			want:     ".backlogit/queue/004-T.md",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := &Workspace{}
			if tc.rootDir != "" {
				ws.Config = &config.WorkspaceConfig{
					QueueLayout: &config.QueueLayoutConfig{RootDir: tc.rootDir},
				}
			}
			got := canonicalRestorePath(ws, tc.basename)
			assert.Equal(t, tc.want, got)
			assert.True(t, strings.HasPrefix(got, ".backlogit/"),
				"restore path must be .backlogit-prefixed to satisfy the F-006 guard")
		})
	}
}
