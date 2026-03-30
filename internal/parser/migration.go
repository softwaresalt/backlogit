package parser

import (
	"context"
	"fmt"
	"os"
)

// Migrate parses a legacy backlog.md and returns the extracted items.
// The caller is responsible for creating artifacts in the workspace.
func Migrate(ctx context.Context, legacyPath string) ([]LegacyItem, error) {
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return nil, fmt.Errorf("read legacy file: %w", err)
	}
	return ParseLegacy(string(data))
}
