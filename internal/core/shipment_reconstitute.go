package core

import "context"

// ReconstitutePlan describes a declarative redistribution of shipment members.
type ReconstitutePlan struct {
	SourceShipments []string                   `json:"source_shipments" yaml:"source_shipments"`
	Shipments       []ReconstituteShipmentSpec `json:"shipments" yaml:"shipments"`
	UnassignedItems []string                   `json:"unassigned_items,omitempty" yaml:"unassigned_items,omitempty"`
	Rewire          []ReconstituteRewireSpec   `json:"rewire,omitempty" yaml:"rewire,omitempty"`
}

// ReconstituteShipmentSpec describes a new or existing target shipment.
type ReconstituteShipmentSpec struct {
	ID            string                  `json:"id" yaml:"id"`
	Label         string                  `json:"label,omitempty" yaml:"label,omitempty"`
	Title         string                  `json:"title,omitempty" yaml:"title,omitempty"`
	Feature       ReconstituteFeatureSpec `json:"feature" yaml:"feature"`
	Items         []string                `json:"items" yaml:"items"`
	BlocksOn      []string                `json:"blocks_on,omitempty" yaml:"blocks_on,omitempty"`
	AutoOrder     *bool                   `json:"auto_order,omitempty" yaml:"auto_order,omitempty"`
	ExplicitOrder bool                    `json:"explicit_order,omitempty" yaml:"explicit_order,omitempty"`
	KeepID        bool                    `json:"keep_id,omitempty" yaml:"keep_id,omitempty"`
}

// ReconstituteFeatureSpec selects an existing feature or requests a new one.
type ReconstituteFeatureSpec struct {
	ID     string                         `json:"id,omitempty" yaml:"id,omitempty"`
	Create *ReconstituteCreateFeatureSpec `json:"create,omitempty" yaml:"create,omitempty"`
}

// ReconstituteCreateFeatureSpec describes a covering feature to create.
type ReconstituteCreateFeatureSpec struct {
	Title       string `json:"title" yaml:"title"`
	Priority    string `json:"priority,omitempty" yaml:"priority,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	RelatedTo   string `json:"related_to,omitempty" yaml:"related_to,omitempty"`
}

// ReconstituteRewireSpec describes replacement sequencing dependencies.
type ReconstituteRewireSpec struct {
	Item     string   `json:"item" yaml:"item"`
	BlocksOn []string `json:"blocks_on" yaml:"blocks_on"`
}

// ReconstituteOptions supplies a plan and execution controls.
type ReconstituteOptions struct {
	PlanPath string `json:"plan_path,omitempty" yaml:"plan_path,omitempty"`
	PlanYAML string `json:"plan_yaml,omitempty" yaml:"plan_yaml,omitempty"`
	DryRun   bool   `json:"dry_run" yaml:"dry_run"`
	Force    bool   `json:"force,omitempty" yaml:"force,omitempty"`
	Reason   string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// ReconstituteDependencyChange reports dependency edges added to or removed from an item.
type ReconstituteDependencyChange struct {
	Item    string   `json:"item" yaml:"item"`
	Added   []string `json:"added" yaml:"added"`
	Removed []string `json:"removed" yaml:"removed"`
}

// ReconstituteCreatedArtifact identifies a predicted or created artifact.
type ReconstituteCreatedArtifact struct {
	ID    string `json:"id" yaml:"id"`
	Label string `json:"label,omitempty" yaml:"label,omitempty"`
	Title string `json:"title" yaml:"title"`
}

// ReconstituteShipmentResult reports a reconstitution outcome or preview.
type ReconstituteShipmentResult struct {
	Status            string                         `json:"status" yaml:"status"`
	DryRun            bool                           `json:"dry_run" yaml:"dry_run"`
	Force             bool                           `json:"force,omitempty" yaml:"force,omitempty"`
	Reason            string                         `json:"reason,omitempty" yaml:"reason,omitempty"`
	CorrelationID     string                         `json:"correlation_id,omitempty" yaml:"correlation_id,omitempty"`
	PlanDigest        string                         `json:"plan_digest" yaml:"plan_digest"`
	SourceShipments   []string                       `json:"source_shipments" yaml:"source_shipments"`
	CreatedFeatures   []ReconstituteCreatedArtifact  `json:"created_features" yaml:"created_features"`
	CreatedShipments  []ReconstituteCreatedArtifact  `json:"created_shipments" yaml:"created_shipments"`
	TargetShipments   []string                       `json:"target_shipments" yaml:"target_shipments"`
	LabelMap          map[string]string              `json:"label_map" yaml:"label_map"`
	AdoptedIDMap      map[string]string              `json:"adopted_id_map" yaml:"adopted_id_map"`
	BeforeManifests   map[string][]string            `json:"before_manifests" yaml:"before_manifests"`
	AfterManifests    map[string][]string            `json:"after_manifests" yaml:"after_manifests"`
	DependencyChanges []ReconstituteDependencyChange `json:"dependency_changes" yaml:"dependency_changes"`
}

// ReconstituteShipments redistributes the members of source shipments across
// target shipments according to a declarative plan. A dry run validates the
// plan and previews the result without writing. A live run is all-or-nothing:
// it journals its preimage, applies every mutation, verifies manifest
// integrity, and rolls back completely on any failure.
func ReconstituteShipments(ctx context.Context, ws *Workspace, opts ReconstituteOptions) (*ReconstituteShipmentResult, error) {
	if ws == nil {
		return nil, reconstituteValidationError("workspace is nil")
	}
	if opts.DryRun {
		if err := ensureNoPendingShipmentRecoveryReadOnly(ws); err != nil {
			return nil, err
		}
		st, err := planReconstitution(ctx, ws, opts)
		if err != nil {
			return nil, err
		}
		return projectReconstitution(ctx, ws, st)
	}
	if ws.DB == nil {
		return nil, reconstituteValidationError("live reconstitution requires an index database")
	}
	return executeReconstitution(ctx, ws, opts)
}
