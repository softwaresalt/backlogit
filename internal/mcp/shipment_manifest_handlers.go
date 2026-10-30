package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/core"
)

// shipmentManifestOptions extracts the options shared by the manifest re-plan tools.
func shipmentManifestOptions(request mcplib.CallToolRequest) core.ShipmentManifestOptions {
	args := request.Params.Arguments
	reason, _ := args["reason"].(string)
	force, _ := args["force"].(bool)
	dryRun, _ := args["dry_run"].(bool)
	return core.ShipmentManifestOptions{Force: force, Reason: reason, DryRun: dryRun}
}

func (s *Server) handleRemoveFromShipment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
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

	result, err := core.RemoveFromShipment(ctx, s.Workspace, shipmentID, itemID, shipmentManifestOptions(request))
	if err != nil {
		return domainError("remove from shipment", err), nil
	}
	return toolResultJSON(result)
}

func (s *Server) handleMoveShipmentItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
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

	result, err := core.MoveShipmentItem(ctx, s.Workspace, itemID, fromID, toID, shipmentManifestOptions(request))
	if err != nil {
		return domainError("move shipment item", err), nil
	}
	return toolResultJSON(result)
}

func (s *Server) handleReorderShipment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	shipmentID, _ := request.Params.Arguments["shipment_id"].(string)
	if shipmentID == "" {
		return ValidationFailed("shipment_id is required"), nil
	}
	items, _ := request.Params.Arguments["items"].(string)
	auto, _ := request.Params.Arguments["auto"].(bool)
	opts := shipmentManifestOptions(request)
	opts.Items = splitCommaSeparated(items)
	opts.Auto = auto
	logger.Info("shipment tool invoked", "tool", "backlogit_reorder_shipment",
		"shipment_id", shipmentID, "auto", auto)

	result, err := core.ReorderShipment(ctx, s.Workspace, shipmentID, opts)
	if err != nil {
		return domainError("reorder shipment", err), nil
	}
	return toolResultJSON(result)
}
