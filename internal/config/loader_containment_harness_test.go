package config_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

func TestSE7aConfigLoadContainmentHarness(t *testing.T) {
	t.Run("rejects root dir lexical escape", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, config.WriteDefaults(dir))

		cfgPath := filepath.Join(dir, "config.yaml")
		raw, err := os.ReadFile(cfgPath)
		require.NoError(t, err)
		escaped := strings.Replace(string(raw), "root_dir: queue", "root_dir: ..\\..\\outside", 1)
		require.NoError(t, os.WriteFile(cfgPath, []byte(escaped), 0o644))

		_, err = config.Load(context.Background(), dir)
		if err == nil {
			t.Fatalf("TODO: implement SE-7a config-load containment for QueueLayout.RootDir")
		}
		require.ErrorContains(t, err, "outside the workspace")
	})

	t.Run("loads legitimate in-workspace config", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, config.WriteDefaults(dir))

		_, err := config.Load(context.Background(), dir)
		require.NoError(t, err)
	})
}
