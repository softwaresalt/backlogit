package integration_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func checkUniqueYAMLMappingKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]struct{}, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i].Value
			if _, exists := seen[k]; exists {
				return fmt.Errorf("duplicate YAML key %q at line %d", k, node.Content[i].Line)
			}
			seen[k] = struct{}{}
			if err := checkUniqueYAMLMappingKeys(node.Content[i+1]); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range node.Content {
		if err := checkUniqueYAMLMappingKeys(child); err != nil {
			return err
		}
	}
	return nil
}

func TestU2_WorkspaceConfigCanonicalPlansDir(t *testing.T) {
	repoRoot := testRepoRoot(t)
	raw := readGovernedSurface(t, repoRoot, ".autoharness/config.yaml")

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(raw), &doc))
	require.NoError(t, checkUniqueYAMLMappingKeys(&doc))

	var cfg map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(raw), &cfg))

	assert.Equal(t, "1.3.0", cfg["schema_version"])
	assert.Equal(t, "full", cfg["preset"])

	backlogCfg, ok := cfg["backlog"].(map[string]any)
	require.True(t, ok, "backlog section must exist")
	assert.Equal(t, "backlogit", backlogCfg["tool"])
	assert.Equal(t, ".backlogit", backlogCfg["directory"])

	docsCfg, ok := cfg["docs"].(map[string]any)
	require.True(t, ok, "docs section must exist")
	assert.Equal(t, "docs", docsCfg["root"])

	subdirs, ok := docsCfg["subdirectories"].(map[string]any)
	require.True(t, ok, "docs.subdirectories section must exist")
	assert.Equal(t, "plans", subdirs["plans"])
	assert.Equal(t, "compound", subdirs["compound"])
	assert.Equal(t, "decisions", subdirs["decisions"])
	assert.Equal(t, "memory", subdirs["memory"])
	assert.Equal(t, "closure", subdirs["closure"])
	assert.Equal(t, "design-docs", subdirs["design_docs"])
	assert.Equal(t, "product-specs", subdirs["product_specs"])
}
