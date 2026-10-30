package mcp

import (
	"context"
	"errors"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// shipmentManifestOptions extracts the options shared by the manifest re-plan tools.
func shipmentManifestOptions(request mcplib.CallToolRequest) core.ShipmentManifestOptions {
	args := request.Params.Arguments
	reason, _ := args["reason"].(string)
	force, _ := args["force"].(bool)
	dryRun, _ := args["dry_run"].(bool)
	return core.ShipmentManifestOptions{Force: force, Reason: reason, DryRun: dryRun}
}

// manifestWorkspace returns the workspace for a manifest tool. A dry-run uses
// the already-initialized workspace when present; otherwise it opens a
// transient read-only, non-recovering workspace (never cached on the server) so
// a preview cannot run recovery or write SQLite. The returned release func must
// be called when the handler is done.
func (s *Server) manifestWorkspace(ctx context.Context, dryRun bool) (*core.Workspace, func(), *mcplib.CallToolResult) {
	noop := func() {}
	if !dryRun {
		ws, result := s.requireWorkspace(ctx)
		return ws, noop, result
	}
	s.workspaceMu.Lock()
	cached := s.Workspace
	// Capture the storage root while still holding workspaceMu so no
	// unlocked read of s.Workspace races with ensureWorkspace.
	storageRoot := s.backlogitDir()
	s.workspaceMu.Unlock()
	if cached != nil {
		return cached, noop, nil
	}
	if !dirExists(storageRoot) {
		return nil, noop, WorkspaceNotInitialized()
	}
	ws, err := core.NewReadOnlyWorkspace(ctx, s.RootPath)
	if err != nil {
		var ambiguous *corerrors.AmbiguousWorkspaceRootError
		if errors.As(err, &ambiguous) {
			return nil, noop, WorkspaceRootAmbiguous(ambiguous.Roots)
		}
		return nil, noop, InternalError(fmt.Sprintf("open workspace: %v", err))
	}
	return ws, func() {
		if cerr := ws.Close(); cerr != nil {
			logger.Warn("close read-only workspace", "error", cerr)
		}
	}, nil
}

func (s *Server) handleRemoveFromShipment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	opts := shipmentManifestOptions(request)
	ws, release, wsResult := s.manifestWorkspace(ctx, opts.DryRun)
	if wsResult != nil {
		return wsResult, nil
	}
	defer release()
	shipmentID, _ := request.Params.Arguments["shipment_id"].(string)
	if shipmentID == "" {
		return ValidationFailed("shipment_id is required"), nil
	}
	itemID, _ := request.Params.Arguments["item_id"].(string)
	if itemID == "" {
		return ValidationFailed("item_id is required"), nil
	}
	logger.Info("shipment tool invoked", "tool", "backlogit_remove_from_shipment",
		"shipment_id", shipmentID, "item_id", itemID)

	result, err := core.RemoveFromShipment(ctx, ws, shipmentID, itemID, opts)
	if err != nil {
		return domainError("remove from shipment", err), nil
	}
	return toolResultJSON(result)
}

func (s *Server) handleMoveShipmentItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	opts := shipmentManifestOptions(request)
	ws, release, wsResult := s.manifestWorkspace(ctx, opts.DryRun)
	if wsResult != nil {
		return wsResult, nil
	}
	defer release()
	itemID, _ := request.Params.Arguments["item_id"].(string)
	if itemID == "" {
		return ValidationFailed("item_id is required"), nil
	}
	fromID, _ := request.Params.Arguments["from_shipment_id"].(string)
	if fromID == "" {
		return ValidationFailed("from_shipment_id is required"), nil
	}
	toID, _ := request.Params.Arguments["to_shipment_id"].(string)
	if toID == "" {
		return ValidationFailed("to_shipment_id is required"), nil
	}
	logger.Info("shipment tool invoked", "tool", "backlogit_move_shipment_item",
		"item_id", itemID, "from_shipment_id", fromID, "to_shipment_id", toID)

	result, err := core.MoveShipmentItem(ctx, ws, itemID, fromID, toID, opts)
	if err != nil {
		return domainError("move shipment item", err), nil
	}
	return toolResultJSON(result)
}

func (s *Server) handleReorderShipment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	opts := shipmentManifestOptions(request)
	ws, release, wsResult := s.manifestWorkspace(ctx, opts.DryRun)
	if wsResult != nil {
		return wsResult, nil
	}
	defer release()
	shipmentID, _ := request.Params.Arguments["shipment_id"].(string)
	if shipmentID == "" {
		return ValidationFailed("shipment_id is required"), nil
	}
	items, _ := request.Params.Arguments["items"].(string)
	auto, _ := request.Params.Arguments["auto"].(bool)
	opts.Items = splitCommaSeparated(items)
	opts.Auto = auto
	logger.Info("shipment tool invoked", "tool", "backlogit_reorder_shipment",
		"shipment_id", shipmentID, "auto", auto)

	result, err := core.ReorderShipment(ctx, ws, shipmentID, opts)
	if err != nil {
		return domainError("reorder shipment", err), nil
	}
	return toolResultJSON(result)
}
