package toolchain

import (
	"context"
	"fmt"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// VerifyLauncherScripts verifies the workspace launcher scripts.
func VerifyLauncherScripts(rootPath string) ([]ToolVerificationFinding, error) {
	return nil, fmt.Errorf("verify launcher scripts: %w", corerrors.ErrNotImplemented)
}

// AuditRepositoryToolchain audits the repository toolchain.
func AuditRepositoryToolchain(ctx context.Context, rootPath string) (*ToolchainVerificationReport, error) {
	return nil, fmt.Errorf("audit repository toolchain: %w", corerrors.ErrNotImplemented)
}

// WriteToolchainManifest writes the workspace toolchain manifest.
func WriteToolchainManifest(rootPath string, manifest *ToolchainManifest) error {
	return fmt.Errorf("write toolchain manifest: %w", corerrors.ErrNotImplemented)
}

// RenderPinnedMCPConfig renders the workspace's pinned MCP configuration.
func RenderPinnedMCPConfig(rootPath string, manifest *ToolchainManifest) ([]byte, error) {
	return nil, fmt.Errorf("render pinned MCP config: %w", corerrors.ErrNotImplemented)
}
