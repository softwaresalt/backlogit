package core

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/mdfront"
	"github.com/softwaresalt/backlogit/internal/models"
)

// SetArtifactComplexity sets or clears the task complexity metadata using a
// body-preserving frontmatter seam.
func SetArtifactComplexity(ctx context.Context, ws *Workspace, id, complexity string) (artifact *models.Artifact, retErr error) {
	unlock, err := lockArtifactMutation(ctx, ws, id)
	if err != nil {
		if errors.Is(err, ErrTaskBusy) {
			return nil, err
		}
		return nil, fmt.Errorf("lock task %s: %w", id, err)
	}
	defer func() {
		if err := unlock(); err != nil && retErr == nil {
			retErr = fmt.Errorf("unlock task %s: %w", id, err)
		}
	}()

	path, err := FindArtifactPath(ctx, ws, id)
	if err != nil {
		return nil, fmt.Errorf("find artifact %s: %w", id, err)
	}
	ioPath, err := resolveContainedArtifactPath(ws, path)
	if err != nil {
		return nil, fmt.Errorf("resolve contained path for %s: %w", id, err)
	}

	raw, err := os.ReadFile(ioPath)
	if err != nil {
		return nil, fmt.Errorf("read artifact %s: %w", id, err)
	}
	mdDoc, err := mdfront.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("decode artifact %s: %w", id, err)
	}
	if !mdDoc.HasFrontmatter {
		return nil, fmt.Errorf("artifact %s has no frontmatter", id)
	}

	artifactType, _ := mdDoc.Frontmatter["artifact_type"].(string)
	if err := validateComplexityMutation(ws, artifactType, complexity); err != nil {
		return nil, fmt.Errorf("validate complexity for artifact %s: %w", id, err)
	}
	if complexity != "" {
		setDecodedCustomField(mdDoc, "complexity", complexity)
	} else {
		clearDecodedCustomField(mdDoc, "complexity")
	}

	out, err := mdDoc.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode artifact %s: %w", id, err)
	}
	if err := writeComplexitySeamDurable(ioPath, out, WorkspaceDurableWrites(ws)); err != nil {
		return nil, fmt.Errorf("write artifact %s: %w", id, err)
	}

	artifact, err = models.ArtifactFromFrontmatter(mdDoc.Frontmatter, string(mdDoc.Body))
	if err != nil {
		return nil, fmt.Errorf("reconstruct artifact %s: %w", id, err)
	}
	if ws.DB != nil {
		if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
			return nil, fmt.Errorf("upsert artifact %s: %w", id, err)
		}
	}
	return artifact, nil
}

// ValidateComplexityValue confirms complexity is a member of the type's
// header-def complexity enum.
func ValidateComplexityValue(ws *Workspace, artifactType, complexity string) error {
	return validateComplexityMutation(ws, artifactType, complexity)
}

func validateComplexityMutation(ws *Workspace, artifactType, complexity string) error {
	if artifactType != "task" {
		return fmt.Errorf("complexity is task-only; artifact type %q cannot store complexity: %w", artifactType, blerrors.ErrValidation)
	}
	if ws.HeaderDef == nil {
		return fmt.Errorf("cannot validate complexity: header-def not loaded: %w", blerrors.ErrConfig)
	}
	schema, err := ws.HeaderDef.ResolveFieldSchema(artifactType)
	if err != nil {
		return fmt.Errorf("resolve schema for %q: %w", artifactType, err)
	}
	def, ok := schema["complexity"]
	if !ok {
		return fmt.Errorf("artifact type %q does not define a complexity field: %w", artifactType, blerrors.ErrValidation)
	}
	if complexity == "" {
		return nil
	}
	for _, v := range def.Values {
		if v == complexity {
			return nil
		}
	}
	return fmt.Errorf("invalid complexity %q: must be one of %v: %w", complexity, def.Values, blerrors.ErrValidation)
}

func clearDecodedCustomField(mdDoc *mdfront.Markdown, key string) {
	cf, ok := mdDoc.Frontmatter["custom_fields"].(map[string]any)
	if !ok || cf == nil {
		return
	}
	delete(cf, key)
	if len(cf) == 0 {
		delete(mdDoc.Frontmatter, "custom_fields")
		return
	}
	mdDoc.Frontmatter["custom_fields"] = cf
}

func writeComplexitySeamDurable(path string, data []byte, durable bool) error {
	err := atomicfile.WriteFileAtomicWithOptions(path, data, atomicfile.Options{DurableWrites: durable})
	if err == nil {
		return nil
	}
	if blerrors.IsWriteNotApplied(err) {
		return atomicfile.WriteFileAtomicWithOptions(path, data, atomicfile.Options{DurableWrites: durable})
	}
	return err
}
