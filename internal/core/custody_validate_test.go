package core

import (
	"errors"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const u3ValidateSource = "custody_validate.go"

// TestU3_CustodyValidationAndApprovalTokenVerification is the RED harness for
// workflow-custody/v1 validation, Lstat path containment, and Profile A
// approval-token verification (210.003-T).
//
// Behavioral subtests call only the U2 signatures declared today, so this file
// compiles before custody_validate.go exists and each behavioral case fails on
// an assertion against the ErrNotImplemented stubs. Cases that need populated
// envelope, entry, or proof fields, a trust-anchor parameter, or a read seam are
// source-shape: they parse custody_validate.go and custody_types.go with go/parser
// and inspect them with go/ast, so they fail on an assertion rather than a build.
func TestU3_CustodyValidationAndApprovalTokenVerification(t *testing.T) {
	t.Run("path_containment_and_symlink_reparse_rejection", func(t *testing.T) {
		t.Run("accepts_repo_relative_posix_paths", func(t *testing.T) {
			ws := u3Workspace(t)
			require.NoError(t, ValidateCustodyEntryPath(ws, "docs/plans/2026-11-02-plan.md"))
			require.NoError(t, ValidateCustodyEntryPath(ws, "internal/core/custody.go"))
		})

		t.Run("accepts_relative_root_dot", func(t *testing.T) {
			t.Chdir(u3Workspace(t).RootPath)
			require.NoError(t, ValidateCustodyEntryPath(&Workspace{RootPath: "."}, "docs/plans/2026-11-02-plan.md"))
		})

		rejected := []struct {
			name string
			path string
		}{
			{"empty", ""},
			{"parent_traversal", "../escape.md"},
			{"embedded_traversal", "docs/../../escape.md"},
			{"absolute_posix", "/etc/passwd"},
			{"backslash_separator", `docs\plans\plan.md`},
			{"drive_letter", "C:/Windows/win.ini"},
			{"unc_prefix", `\\server\share\plan.md`},
			{"empty_segment", "docs//plan.md"},
			{"git_directory", ".git/config"},
		}
		for _, c := range rejected {
			t.Run("rejects_"+c.name, func(t *testing.T) {
				u3RequireRejected(t, ValidateCustodyEntryPath(u3Workspace(t), c.path))
			})
		}

		t.Run("rejects_leaf_symlink", func(t *testing.T) {
			ws := u3Workspace(t)
			u3Symlink(t, "2026-11-02-plan.md", filepath.Join(ws.RootPath, "docs", "plans", "leaf.md"))
			u3RequireRejected(t, ValidateCustodyEntryPath(ws, "docs/plans/leaf.md"))
		})

		t.Run("rejects_intermediate_symlink", func(t *testing.T) {
			ws := u3Workspace(t)
			u3Symlink(t, "internal", filepath.Join(ws.RootPath, "linkdir"))
			u3RequireRejected(t, ValidateCustodyEntryPath(ws, "linkdir/core/custody.go"))
		})

		t.Run("rejects_symlink_escaping_workspace", func(t *testing.T) {
			ws := u3Workspace(t)
			u3Symlink(t, t.TempDir(), filepath.Join(ws.RootPath, "escape"))
			u3RequireRejected(t, ValidateCustodyEntryPath(ws, "escape/secret.md"))
		})

		t.Run("entry_path_walks_components_with_lstat", func(t *testing.T) {
			file := u3ValidateFile(t)
			fn := u3FuncDecl(t, file, u3ValidateSource, "ValidateCustodyEntryPath")
			for _, name := range []string{"Abs", "EvalSymlinks", "Lstat", "IsSymlinkOrReparsePoint"} {
				u3ExpectIdent(t, fn, "ValidateCustodyEntryPath", name)
			}
		})

		t.Run("handle_pinned_no_follow_read_declared", func(t *testing.T) {
			file := u3ValidateFile(t)
			if !u3Mentions(file, "O_NOFOLLOW") && !u3Mentions(file, "FILE_FLAG_OPEN_REPARSE_POINT") {
				t.Errorf("%s declares no handle-pinned no-follow open (O_NOFOLLOW or FILE_FLAG_OPEN_REPARSE_POINT)", u3ValidateSource)
			}
		})
	})

	t.Run("atomic_rename_case_only_rename_and_unsupported_kind_guards", func(t *testing.T) {
		t.Run("rejects_zero_value_envelope", func(t *testing.T) {
			u3RequireRejected(t, ValidateCustodyEnvelope(WorkflowCustodyEnvelope{}))
		})

		t.Run("rename_requires_old_path_digest_or_rename_group", func(t *testing.T) {
			file := u3ValidateFile(t)
			u3ExpectLiteral(t, file, u3ValidateSource, "renamed")
			u3ExpectLiteral(t, file, u3ValidateSource, "old_base_blob_sha256")
			u3ExpectLiteral(t, file, u3ValidateSource, "rename_group_id")
		})

		t.Run("detects_case_only_rename", func(t *testing.T) {
			file := u3ValidateFile(t)
			u3ExpectIdent(t, file, u3ValidateSource, "EqualFold")
			u3ExpectLiteral(t, file, u3ValidateSource, "case_only_rename")
		})

		t.Run("rejects_partial_rename_selection", func(t *testing.T) {
			u3ExpectIdent(t, u3ValidateFile(t), u3ValidateSource, "ErrPartialRenameAdoptionRejected")
		})

		t.Run("rejects_unmodeled_change_kinds", func(t *testing.T) {
			file := u3ValidateFile(t)
			u3ExpectIdent(t, file, u3ValidateSource, "ErrUnsupportedCustodyChangeKind")
			u3ExpectLiteral(t, file, u3ValidateSource, "unmerged")
			u3ExpectLiteral(t, file, u3ValidateSource, "copied")
		})
	})

	t.Run("mixed_index_worktree_divergence_and_duplicate_key_guards", func(t *testing.T) {
		t.Run("rejects_unreconciled_mixed_divergence", func(t *testing.T) {
			u3ExpectIdent(t, u3ValidateFile(t), u3ValidateSource, "ErrMixedIndexWorktreeDivergence")
		})

		t.Run("accepts_only_operator_verified_mixed_reconciled", func(t *testing.T) {
			file := u3ValidateFile(t)
			u3ExpectLiteral(t, file, u3ValidateSource, "staged_diverges_accepted")
			u3ExpectLiteral(t, file, u3ValidateSource, "mixed_reconciled")
			u3ExpectLiteral(t, file, u3ValidateSource, "operator_verified")
		})

		t.Run("rejects_duplicate_json_and_yaml_keys", func(t *testing.T) {
			file := u3ValidateFile(t)
			u3ExpectIdent(t, file, u3ValidateSource, "SimpleFold")
			u3ExpectLiteral(t, file, u3ValidateSource, "duplicate")
		})
	})

	t.Run("profile_a_custody_approval_token_mint_verify_and_expiry_bounds", func(t *testing.T) {
		t.Run("request_identity_digest_is_deterministic", func(t *testing.T) {
			d1, err1 := CustodyRequestIdentityDigest(WorkflowCustodyEnvelope{})
			d2, err2 := CustodyRequestIdentityDigest(WorkflowCustodyEnvelope{})
			require.NotErrorIs(t, err1, blerrors.ErrNotImplemented)
			require.NotErrorIs(t, err2, blerrors.ErrNotImplemented)
			assert.Equal(t, d1, d2)
			if err1 == nil {
				assert.NotEmpty(t, d1)
			}
		})

		t.Run("digest_is_domain_separated", func(t *testing.T) {
			fn := u3FuncDecl(t, u3ValidateFile(t), u3ValidateSource, "CustodyRequestIdentityDigest")
			u3ExpectIdent(t, fn, "CustodyRequestIdentityDigest", "CustodyRequestIdentityDomain")
		})

		t.Run("mint_returns_token_or_fails_closed", func(t *testing.T) {
			token, err := MintCustodyApprovalToken(CustodyApprovalProof{})
			require.NotErrorIs(t, err, blerrors.ErrNotImplemented)
			if err == nil {
				assert.NotEmpty(t, token)
			}
		})

		t.Run("verify_rejects_empty_and_forged_tokens", func(t *testing.T) {
			tokens := []struct {
				name  string
				token string
			}{
				{"empty_token", ""},
				{"malformed_token", "not-a-token"},
				{"forged_shape_token", "v1.AAAA.BBBB"},
			}
			for _, c := range tokens {
				t.Run(c.name, func(t *testing.T) {
					_, err := VerifyCustodyApprovalToken(c.token, WorkflowCustodyEnvelope{}, time.Now())
					require.Error(t, err)
					require.NotErrorIs(t, err, blerrors.ErrNotImplemented)
					assert.True(t,
						errors.Is(err, ErrAuthorizationRequired) ||
							errors.Is(err, ErrApprovalChannelUnavailable) ||
							errors.Is(err, ErrOperatorApprovalRequired),
						"want a fail-closed approval sentinel, got %v", err)
				})
			}
		})

		t.Run("verify_enforces_operator_role_window_and_algorithm", func(t *testing.T) {
			fn := u3FuncDecl(t, u3ValidateFile(t), u3ValidateSource, "VerifyCustodyApprovalToken")
			u3ExpectLiteral(t, fn, "VerifyCustodyApprovalToken", "operator")
			u3ExpectIdent(t, fn, "VerifyCustodyApprovalToken", "MaxCustodyApprovalTokenWindow")
			if !u3Mentions(fn, "ed25519") && !u3Mentions(fn, "hmac") {
				t.Errorf("VerifyCustodyApprovalToken verifies no Ed25519 or HMAC signature")
			}
		})

		t.Run("approval_proof_declares_bound_fields", func(t *testing.T) {
			types := parseCustodySource(t, "custody_types.go")
			if types == nil {
				t.FailNow()
			}
			st, ok := indexCustodyDecls(types).structs["CustodyApprovalProof"]
			if !ok {
				t.Fatalf("CustodyApprovalProof is not declared in custody_types.go")
			}
			bound := []string{
				"key_id", "algorithm", "principal", "authorized_role", "workflow_id",
				"shipment_id", "scope_branch", "candidate_set_digest",
				"not_before_utc", "not_after_utc", "nonce",
			}
			for _, name := range bound {
				if !u3HasJSONField(st, name) {
					t.Errorf("CustodyApprovalProof declares no field with json tag %q in custody_types.go", name)
				}
			}
		})
	})

	t.Run("verify_token_takes_trust_anchor", func(t *testing.T) {
		fn := u3FuncDecl(t, u3ValidateFile(t), u3ValidateSource, "VerifyCustodyApprovalToken")
		for _, p := range fn.Type.Params.List {
			if u3MentionsSub(p.Type, "TrustAnchor") {
				return
			}
		}
		t.Errorf("VerifyCustodyApprovalToken in %s takes no TrustAnchor parameter (invariant I2)", u3ValidateSource)
	})
}

// u3Workspace builds a workspace with the repo-relative files the path cases use.
func u3Workspace(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{"docs/plans/2026-11-02-plan.md", "internal/core/custody.go"} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("placeholder\n"), 0o600))
	}
	return &Workspace{RootPath: root}
}

// u3RequireRejected expects a real rejection, not the not-implemented stub.
func u3RequireRejected(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.NotErrorIs(t, err, blerrors.ErrNotImplemented)
}

// u3Symlink creates a symlink, skipping when the host cannot create one.
func u3Symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation unavailable on this host: %v", err)
	}
}

// u3ValidateFile parses custody_validate.go, the file the GREEN phase creates.
func u3ValidateFile(t *testing.T) *ast.File {
	t.Helper()
	file := parseCustodySource(t, u3ValidateSource)
	if file == nil {
		t.FailNow()
	}
	return file
}

// u3FuncDecl returns the receiver-less function name declared in file.
func u3FuncDecl(t *testing.T, file *ast.File, fileName, name string) *ast.FuncDecl {
	t.Helper()
	fn, ok := indexCustodyDecls(file).funcs[name]
	if !ok {
		t.Fatalf("%s is not declared in %s", name, fileName)
	}
	return fn
}

// u3Mentions reports whether node contains an identifier exactly named name.
func u3Mentions(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}

// u3MentionsSub reports whether node contains an identifier whose name includes substr.
func u3MentionsSub(node ast.Node, substr string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && strings.Contains(id.Name, substr) {
			found = true
		}
		return !found
	})
	return found
}

// u3HasLiteral reports whether node contains a string literal containing substr.
func u3HasLiteral(node ast.Node, substr string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(lit.Value, substr) {
			found = true
		}
		return !found
	})
	return found
}

// u3HasJSONField reports whether st declares a field with the given json tag name.
func u3HasJSONField(st *ast.StructType, jsonName string) bool {
	for _, f := range st.Fields.List {
		if f.Tag == nil {
			continue
		}
		tag, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			continue
		}
		if strings.Contains(tag, `json:"`+jsonName+`"`) || strings.Contains(tag, `json:"`+jsonName+`,`) {
			return true
		}
	}
	return false
}

// u3ExpectIdent records a failure when where does not reference the identifier name.
func u3ExpectIdent(t *testing.T, node ast.Node, where, name string) {
	t.Helper()
	if !u3Mentions(node, name) {
		t.Errorf("%s does not reference %s", where, name)
	}
}

// u3ExpectLiteral records a failure when where has no string literal containing substr.
func u3ExpectLiteral(t *testing.T, node ast.Node, where, substr string) {
	t.Helper()
	if !u3HasLiteral(node, substr) {
		t.Errorf("%s has no string literal containing %q", where, substr)
	}
}
