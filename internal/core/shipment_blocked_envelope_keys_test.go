package core

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestUCYA7_CanonicalBlockedEnvelopeKeys asserts the single canonical blocked
// envelope key list. The list must be declared in shipment_blocked_envelope.go,
// must be the only copy, and must be read by every consumer that validates or
// rewrites the blocked envelope.
func TestUCYA7_CanonicalBlockedEnvelopeKeys(t *testing.T) {
	want := [6]string{
		"blocked_reason",
		"blocked_at",
		"member_status_snapshot",
		"branch",
		"blocked_by",
		"resume_checkpoint_ref",
	}
	assert.Equal(t, want, blockedShipmentEnvelopeKeys(),
		"the canonical blocked envelope key list must contain exactly the six keys in order")

	assert.True(t, coreFileDeclaresFunc(t, "shipment_blocked_envelope.go", "blockedShipmentEnvelopeKeys"),
		"blockedShipmentEnvelopeKeys is not declared in shipment_blocked_envelope.go")
	assert.False(t, coreFileDeclaresFunc(t, "artifacts.go", "blockedShipmentEnvelopeKeys"),
		"blockedShipmentEnvelopeKeys must no longer be declared in artifacts.go")

	decls := coreFuncDecls(t)
	consumers := []string{
		"validateBlockedShipmentEnvelope",
		"UpdateArtifactWithGate",
		"writeArtifactFileGoverned",
		"BlockShipment",
		"shipmentRecoveryCandidates",
	}
	for _, name := range consumers {
		fn, ok := decls[name]
		if !assert.True(t, ok, "consumer %s is not declared in internal/core", name) {
			continue
		}
		assert.True(t, funcCallsIdent(fn, "blockedShipmentEnvelopeKeys"),
			"%s must read the canonical blockedShipmentEnvelopeKeys list", name)
	}

	artifactsSource, err := os.ReadFile("artifacts.go")
	require.NoError(t, err)
	assert.NotContains(t, string(artifactsSource), "deferred to stash 8AF55264",
		"the stale stash deferral comment must be removed from artifacts.go")
	assert.True(t, fileCommentGroupContainsAll(t, "artifacts.go", "pre-update hook", "submitted request"),
		"artifacts.go must document that pre-update hooks keep seeing the submitted request")
}

// TestUCYA7_RefuseBlockedShipmentArtifactTypeFlip asserts that an ungoverned
// artifact write which flips artifact_type into or out of shipment while a
// shipment is blocked is refused with ErrShipmentBlockedRequiresEnvelope.
func TestUCYA7_RefuseBlockedShipmentArtifactTypeFlip(t *testing.T) {
	t.Run("out of shipment while blocked", func(t *testing.T) {
		ctx := context.Background()
		_, ws := setupUR3Workspace(t)
		t.Cleanup(func() { require.NoError(t, ws.Close()) })
		fixture := newURBlockedActiveFixture(t, ws)
		blocked, err := BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{
			Reason:    "u-cya7 artifact type flip out",
			BlockedBy: "UCYA7",
		})
		require.NoError(t, err)
		shipmentPath, err := FindArtifactPath(ctx, ws, blocked.ID)
		require.NoError(t, err)

		flipped := cloneArtifact(loadURCanonicalArtifact(t, ws, blocked.ID))
		flipped.ArtifactType = "task"
		assert.ErrorIs(t, WriteArtifactFileWithOptions(flipped, shipmentPath, false),
			blerrors.ErrShipmentBlockedRequiresEnvelope)
	})

	t.Run("into shipment while blocked", func(t *testing.T) {
		ctx := context.Background()
		_, ws := setupUR3Workspace(t)
		t.Cleanup(func() { require.NoError(t, ws.Close()) })
		fixture := newURBlockedActiveFixture(t, ws)
		memberID := fixture.members[0].ID
		memberPath, err := FindArtifactPath(ctx, ws, memberID)
		require.NoError(t, err)

		flipped := cloneArtifact(loadURCanonicalArtifact(t, ws, memberID))
		flipped.ArtifactType = "shipment"
		flipped.Status = models.StatusBlocked
		assert.ErrorIs(t, WriteArtifactFileWithOptions(flipped, memberPath, false),
			blerrors.ErrShipmentBlockedRequiresEnvelope)
	})
}

// coreFuncDecls parses every non-test Go file in the package and returns its
// receiver-less function declarations keyed by name.
func coreFuncDecls(t *testing.T) map[string]*ast.FuncDecl {
	t.Helper()

	paths, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	decls := make(map[string]*ast.FuncDecl)
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, parseErr, "parse %s", path)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			decls[fn.Name.Name] = fn
		}
	}
	return decls
}

// coreFileDeclaresFunc reports whether the named source file declares a
// receiver-less function with the given name.
func coreFileDeclaresFunc(t *testing.T, fileName, funcName string) bool {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), fileName, nil, 0)
	require.NoError(t, err, "parse %s", fileName)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == funcName {
			return true
		}
	}
	return false
}

// funcCallsIdent reports whether fn's body calls the plain identifier callee.
func funcCallsIdent(fn *ast.FuncDecl, callee string) bool {
	if fn.Body == nil {
		return false
	}
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, isIdent := call.Fun.(*ast.Ident); isIdent && ident.Name == callee {
			found = true
		}
		return !found
	})
	return found
}

// fileCommentGroupContainsAll reports whether one comment group in fileName
// contains every phrase, compared case-insensitively.
func fileCommentGroupContainsAll(t *testing.T, fileName string, phrases ...string) bool {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), fileName, nil, parser.ParseComments)
	require.NoError(t, err, "parse %s", fileName)
	for _, group := range file.Comments {
		text := strings.ToLower(strings.Join(strings.Fields(group.Text()), " "))
		matched := true
		for _, phrase := range phrases {
			if !strings.Contains(text, phrase) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
