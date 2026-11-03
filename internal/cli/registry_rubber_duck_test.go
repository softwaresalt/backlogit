package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRubberDuck_F14_CustodyBranchParamRendersAsFlag checks that the custody operations
// declaring a branch param render it as the --branch flag their CLI command accepts.
func TestRubberDuck_F14_CustodyBranchParamRendersAsFlag(t *testing.T) {
	ops := loadRegistryOperations(t)
	for _, name := range []string{"preview_custody", "verify_custody"} {
		op, ok := ops[name]
		require.Truef(t, ok, "registry must declare %s", name)
		require.Equal(t, "branch", op.Params["branch"], "%s must declare the branch param", name)
		assert.Containsf(t, op.CLICommand, "--branch {{branch}}",
			"%s cli_command must render the declared branch param", name)
	}
}
