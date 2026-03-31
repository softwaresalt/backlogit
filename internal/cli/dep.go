package cli

import (
	"github.com/spf13/cobra"
)

// NewDepCmd creates the `backlogit dep` command group for managing dependencies.
//
// Worker: Create cobra command with three subcommands: add, remove, list.
// Each subcommand delegates to the corresponding function in internal/db/dependencies.go.
func NewDepCmd() *cobra.Command {
	panic("not implemented: Worker: Create 'dep' cobra command group with add/remove/list subcommands for dependency management")
}

// NewDepAddCmd creates `backlogit dep add <item-id> <depends-on> [--type blocks]`.
//
// Worker: Parse item-id, depends-on from args. Accept optional --type flag (default "blocks").
// Call db.DetectCycle first, reject if cycle found. Then call db.UpsertDependency.
func NewDepAddCmd() *cobra.Command {
	panic("not implemented: Worker: Create 'dep add' command that validates cycle-free before inserting dependency edge")
}

// NewDepRemoveCmd creates `backlogit dep remove <item-id> <depends-on>`.
//
// Worker: Parse item-id and depends-on from args. Call db.DeleteDependency.
func NewDepRemoveCmd() *cobra.Command {
	panic("not implemented: Worker: Create 'dep remove' command to delete a dependency edge")
}

// NewDepListCmd creates `backlogit dep list <item-id> [--reverse]`.
//
// Worker: Parse item-id from args. If --reverse flag set, call db.GetDependents;
// otherwise call db.GetDependencies. Format as table output.
func NewDepListCmd() *cobra.Command {
	panic("not implemented: Worker: Create 'dep list' command showing upstream or downstream dependencies")
}
