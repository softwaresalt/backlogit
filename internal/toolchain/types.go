// Package toolchain declares the workspace toolchain inventory contract.
package toolchain

import (
	"context"
)

// ToolContainmentClass describes how a tool relates to the workspace boundary.
type ToolContainmentClass string

const (
	// WorkspaceLocalPinned identifies a workspace-local, pinned tool.
	WorkspaceLocalPinned ToolContainmentClass = "workspace-local-pinned"
	// HostPrerequisite identifies a required or optional host tool.
	HostPrerequisite ToolContainmentClass = "host-prerequisite"
	// RemoteDeclared identifies a declared remote tool.
	RemoteDeclared ToolContainmentClass = "remote-declared"
	// BuiltInHost identifies a tool supplied by the host.
	BuiltInHost ToolContainmentClass = "built-in-host"
	// Unsupported identifies a tool outside the supported inventory.
	Unsupported ToolContainmentClass = "unsupported"
	// NonConformant identifies a tool that fails the containment contract.
	NonConformant ToolContainmentClass = "non-conformant"

	// Level1WorkspaceLocalPinned denotes workspace-local pinning without an OS sandbox.
	Level1WorkspaceLocalPinned = "level-1-workspace-local-pinned"
)

// ToolchainManifest describes the declared workspace tool inventory.
type ToolchainManifest struct {
	SchemaVersion              string                  `json:"schema_version" yaml:"schema_version"`
	ContainmentLevel           string                  `json:"containment_level" yaml:"containment_level"`
	OSContainerSandboxDeferred bool                    `json:"os_container_sandbox_deferred" yaml:"os_container_sandbox_deferred"`
	WorkspaceRoot              string                  `json:"workspace_root" yaml:"workspace_root"`
	Tools                      []ToolEntry             `json:"tools" yaml:"tools"`
	HostPrerequisites          []HostPrerequisiteEntry `json:"host_prerequisites" yaml:"host_prerequisites"`
}

// ToolEntry describes a tool and its declared containment.
type ToolEntry struct {
	ID                   string               `json:"id" yaml:"id"`
	Kind                 string               `json:"kind" yaml:"kind"`
	ContainmentClass     ToolContainmentClass `json:"containment_class" yaml:"containment_class"`
	ExecutablePath       string               `json:"executable_path" yaml:"executable_path"`
	EntrypointPath       string               `json:"entrypoint_path" yaml:"entrypoint_path"`
	PinnedVersion        string               `json:"pinned_version" yaml:"pinned_version"`
	SHA256               string               `json:"sha256" yaml:"sha256"`
	LockfilePath         string               `json:"lockfile_path" yaml:"lockfile_path"`
	WorkspaceBindingArgs []string             `json:"workspace_binding_args" yaml:"workspace_binding_args"`
	RemotePolicy         string               `json:"remote_policy" yaml:"remote_policy"`
	Reason               string               `json:"reason" yaml:"reason"`
}

// HostPrerequisiteEntry describes a host-provided prerequisite.
type HostPrerequisiteEntry struct {
	ID       string `json:"id" yaml:"id"`
	Command  string `json:"command" yaml:"command"`
	Purpose  string `json:"purpose" yaml:"purpose"`
	Required bool   `json:"required" yaml:"required"`
	Reason   string `json:"reason" yaml:"reason"`
}

// ToolVerificationFinding records a tool's verification outcome.
type ToolVerificationFinding struct {
	ToolID           string               `json:"tool_id" yaml:"tool_id"`
	ContainmentClass ToolContainmentClass `json:"containment_class" yaml:"containment_class"`
	Status           string               `json:"status" yaml:"status"`
	Code             string               `json:"code" yaml:"code"`
	Message          string               `json:"message" yaml:"message"`
	Remediation      string               `json:"remediation" yaml:"remediation"`
}

// ToolchainVerificationReport summarizes verification of a workspace inventory.
type ToolchainVerificationReport struct {
	Valid                      bool                      `json:"valid" yaml:"valid"`
	ContainmentLevel           string                    `json:"containment_level" yaml:"containment_level"`
	OSContainerSandboxDeferred bool                      `json:"os_container_sandbox_deferred" yaml:"os_container_sandbox_deferred"`
	WorkspaceRoot              string                    `json:"workspace_root" yaml:"workspace_root"`
	Tools                      []ToolVerificationFinding `json:"tools" yaml:"tools"`
	HostPrerequisites          []ToolVerificationFinding `json:"host_prerequisites" yaml:"host_prerequisites"`
	Violations                 int                       `json:"violations" yaml:"violations"`
}

// LoadToolchainManifest loads the workspace inventory or synthesizes one.
func LoadToolchainManifest(rootPath string) (*ToolchainManifest, error) {
	return loadToolchainManifest(rootPath)
}

// DefaultWorkspaceToolchainManifest returns an unverified default inventory.
func DefaultWorkspaceToolchainManifest(rootPath string) *ToolchainManifest {
	return defaultWorkspaceToolchainManifest(rootPath)
}

// VerifyWorkspaceToolchain reports the declared inventory's containment.
func VerifyWorkspaceToolchain(ctx context.Context, rootPath string, manifest *ToolchainManifest) (*ToolchainVerificationReport, error) {
	return verifyWorkspaceToolchain(ctx, rootPath, manifest)
}

// ValidateMCPConfigPinning validates the workspace's MCP configuration.
func ValidateMCPConfigPinning(rootPath string, mcpJSONPath string, manifest *ToolchainManifest) ([]ToolVerificationFinding, error) {
	return validateMCPConfigPinning(rootPath, mcpJSONPath, manifest)
}
