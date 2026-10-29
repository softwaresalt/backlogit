package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/config"
)

// File names inside the storage root used by the file-backed trust-anchor
// adapters.
const (
	trustAnchorConfigFile = "config.yaml"
	// TrustAnchorAuditFile is the append-only JSONL audit log for mutations.
	TrustAnchorAuditFile = "trust_anchor_audit.jsonl"
)

// FileTrustAnchorState is the file-backed TrustAnchorState. It rewrites only
// the trust_anchors key of <Dir>/config.yaml, preserving the rest of the file.
type FileTrustAnchorState struct {
	// Dir is the workspace storage root that contains config.yaml.
	Dir string
}

// LoadAnchors implements TrustAnchorState.
func (s FileTrustAnchorState) LoadAnchors(ctx context.Context) ([]config.TrustAnchor, error) {
	cfg, err := config.Load(ctx, s.Dir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg.TrustAnchors, nil
}

// WriteConfig implements TrustAnchorState: it atomically and durably replaces
// the trust_anchors list.
func (s FileTrustAnchorState) WriteConfig(_ context.Context, anchors []config.TrustAnchor) error {
	path := filepath.Join(s.Dir, trustAnchorConfigFile)
	data, err := os.ReadFile(path) //nolint:gosec // path is the workspace config under the storage root
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("parse config: root is not a mapping")
	}
	var seq yaml.Node
	if err := seq.Encode(anchors); err != nil {
		return fmt.Errorf("encode trust anchors: %w", err)
	}
	root := doc.Content[0]
	replaced := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "trust_anchors" {
			root.Content[i+1] = &seq
			replaced = true
			break
		}
	}
	if !replaced {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "trust_anchors"}, &seq)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := atomicfile.WriteFileAtomicWithOptions(path, out, atomicfile.Options{DurableWrites: true}); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// WriteStatus implements TrustAnchorState by updating one anchor status in
// the persisted trust_anchors list.
func (s FileTrustAnchorState) WriteStatus(ctx context.Context, change TrustAnchorStatusChange) error {
	anchors, err := s.LoadAnchors(ctx)
	if err != nil {
		return err
	}
	found := false
	for i := range anchors {
		if anchors[i].ID == change.ID {
			anchors[i].Status = change.Status
			found = true
		}
	}
	if !found {
		return fmt.Errorf("%w: anchor %q not found for status change", ErrTrustAnchorNotFound, change.ID)
	}
	return s.WriteConfig(ctx, anchors)
}

// FileTrustAnchorAuditLog is the file-backed TrustAnchorAuditor: an
// append-only fsynced JSONL log.
type FileTrustAnchorAuditLog struct {
	// Path is the JSONL file to append to.
	Path string
}

type trustAnchorAuditRecord struct {
	Action      TrustAnchorAction `json:"action"`
	AnchorID    string            `json:"anchor_id"`
	NewAnchorID string            `json:"new_anchor_id,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	Outcome     string            `json:"outcome"`
	Reason      string            `json:"reason,omitempty"`
	Approver    string            `json:"approver,omitempty"`
	At          time.Time         `json:"at"`
}

// RecordAudit implements TrustAnchorAuditor.
func (l FileTrustAnchorAuditLog) RecordAudit(_ context.Context, ev TrustAnchorAuditEvent) error {
	line, err := json.Marshal(trustAnchorAuditRecord(ev))
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	f, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // audit log under the storage root
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close() // best effort; the write error is the failure being reported
		return fmt.Errorf("append audit event: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close() // best effort; the sync error is the failure being reported
		return fmt.Errorf("fsync audit log: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close audit log: %w", err)
	}
	return nil
}

// NewFileTrustAnchorMutator wires a TrustAnchorMutator to the file-backed
// state and audit log under storageRoot.
func NewFileTrustAnchorMutator(storageRoot string, pins ExternalPinSetProvider, approver ApproverVerifier) *TrustAnchorMutator {
	return &TrustAnchorMutator{
		State:    FileTrustAnchorState{Dir: storageRoot},
		Audit:    FileTrustAnchorAuditLog{Path: filepath.Join(storageRoot, TrustAnchorAuditFile)},
		Marker:   FileTrustAnchorMarkerStore{Path: filepath.Join(storageRoot, TrustAnchorMarkerFile)},
		Pins:     pins,
		Approver: approver,
		Now:      time.Now,
	}
}
