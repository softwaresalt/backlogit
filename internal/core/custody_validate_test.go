package core

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/config"
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
			u3ExpectIdent(t, file, u3ValidateSource, "readCustodyWorktreeFileNoFollow")
			u3ExpectIdent(t, u3FuncDecl(t, file, u3ValidateSource, "readCustodyWorktreeFileNoFollow"), "readCustodyWorktreeFileNoFollow", "openCustodyNoFollow")
			for _, platform := range []struct{ name, mechanism string }{
				{"custody_nofollow_unix.go", "O_NOFOLLOW"},
				{"custody_nofollow_windows.go", "FILE_FLAG_OPEN_REPARSE_POINT"},
			} {
				src := parseCustodySource(t, platform.name)
				if src == nil {
					t.FailNow()
				}
				u3ExpectIdent(t, src, platform.name, platform.mechanism)
			}
		})

		t.Run("handle_pinned_read_returns_regular_file_and_refuses_links", func(t *testing.T) {
			ws := u3Workspace(t)
			data, err := readCustodyWorktreeFileNoFollow(ws, "docs/plans/2026-11-02-plan.md")
			require.NoError(t, err)
			assert.Equal(t, "placeholder\n", string(data))

			u3Symlink(t, "2026-11-02-plan.md", filepath.Join(ws.RootPath, "docs", "plans", "leaf.md"))
			data, err = readCustodyWorktreeFileNoFollow(ws, "docs/plans/leaf.md")
			require.ErrorIs(t, err, ErrCustodyPathRejected)
			assert.Nil(t, data)
		})
	})

	t.Run("atomic_rename_case_only_rename_and_unsupported_kind_guards", func(t *testing.T) {
		t.Run("rejects_zero_value_envelope", func(t *testing.T) {
			u3RequireRejected(t, ValidateCustodyEnvelope(WorkflowCustodyEnvelope{}))
		})

		t.Run("accepts_valid_envelope", func(t *testing.T) {
			require.NoError(t, ValidateCustodyEnvelope(u3Envelope()))
		})

		t.Run("rename_requires_complete_old_path_and_digest_pair", func(t *testing.T) {
			cases := []struct {
				name    string
				entry   CustodyEntry
				wantErr error
			}{
				{"complete_rename", u3Rename("docs/new.md", "docs/old.md", "6666", false), nil},
				{"missing_old_path", u3Rename("docs/new.md", "", "6666", false), ErrPartialRenameAdoptionRejected},
				{"missing_old_base_blob", u3Rename("docs/new.md", "docs/old.md", "", false), ErrPartialRenameAdoptionRejected},
				{"case_only_rename_flagged", u3Rename("track-a.md", "Track-A.md", "6666", true), nil},
				{"case_only_flag_missing", u3Rename("track-a.md", "Track-A.md", "6666", false), blerrors.ErrValidation},
				{"identical_old_path_and_path", u3Rename("a.md", "a.md", "6666", false), blerrors.ErrValidation},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					err := ValidateCustodyEnvelope(u3EnvelopeWithEntries(c.entry))
					if c.wantErr == nil {
						require.NoError(t, err)
						return
					}
					require.ErrorIs(t, err, c.wantErr)
				})
			}
		})

		t.Run("partial_rename_pair_rejected", func(t *testing.T) {
			added := u3Entry("docs/new.md")
			added.ChangeKind = "added"
			added.RenameGroupID = "group-1"
			err := ValidateCustodyEnvelope(u3EnvelopeWithEntries(added))
			require.ErrorIs(t, err, ErrPartialRenameAdoptionRejected)

			deleted := u3Entry("docs/old.md")
			deleted.ChangeKind = "deleted"
			deleted.RenameGroupID = "group-1"
			require.NoError(t, ValidateCustodyEnvelope(u3EnvelopeWithEntries(added, deleted)))
		})

		t.Run("unsupported_change_kinds_fail_closed", func(t *testing.T) {
			for _, kind := range []string{"unmerged", "copied"} {
				entry := u3Entry("docs/plans/plan.md")
				entry.ChangeKind = kind
				require.ErrorIs(t, ValidateCustodyEnvelope(u3EnvelopeWithEntries(entry)), ErrUnsupportedCustodyChangeKind, kind)
			}
		})

		t.Run("rejects_case_folded_duplicate_entry_paths", func(t *testing.T) {
			first := u3Entry("Docs/A.md")
			second := u3Entry("docs/a.md")
			require.ErrorIs(t, ValidateCustodyEnvelope(u3EnvelopeWithEntries(first, second)), blerrors.ErrValidation)
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

		t.Run("divergence_rejected_by_default", func(t *testing.T) {
			entry := u3Entry("docs/plans/plan.md")
			entry.IndexSHA256, entry.WorktreeSHA256 = "bbbb", "cccc"
			require.ErrorIs(t, ValidateCustodyEnvelope(u3EnvelopeWithEntries(entry)), ErrMixedIndexWorktreeDivergence)
		})

		t.Run("divergence_accepted_only_under_operator_verified_mixed_reconciled", func(t *testing.T) {
			cases := []struct {
				name        string
				profile     string
				kind        string
				stagedState string
				contributor string
				wantErr     error
			}{
				{"operator_verified_mixed_reconciled", "operator_verified", "preexisting_adoption", "staged_diverges_accepted", "mixed_reconciled", nil},
				{"agent_profile_rejected", "agent_workflow_handoff", "stage_to_ship_handoff", "staged_diverges_accepted", "mixed_reconciled", ErrMixedIndexWorktreeDivergence},
				{"unreconciled_staged_state", "operator_verified", "preexisting_adoption", "unstaged_only", "mixed_reconciled", ErrMixedIndexWorktreeDivergence},
				{"non_mixed_contributor", "operator_verified", "preexisting_adoption", "staged_diverges_accepted", "operator", ErrMixedIndexWorktreeDivergence},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					entry := u3Entry("docs/plans/plan.md")
					entry.IndexSHA256, entry.WorktreeSHA256 = "bbbb", "cccc"
					entry.StagedState = c.stagedState
					entry.Contributor = c.contributor
					env := u3EnvelopeWithEntries(entry)
					env.Receipts[0].AuthorityProfile = c.profile
					env.Receipts[0].Kind = c.kind
					err := ValidateCustodyEnvelope(env)
					if c.wantErr == nil {
						require.NoError(t, err)
						return
					}
					require.ErrorIs(t, err, c.wantErr)
				})
			}
		})

		t.Run("duplicate_json_members_rejected", func(t *testing.T) {
			cases := []struct{ name, raw string }{
				{"exact_duplicate", `{"schema_version":"workflow-custody/v1","schema_version":"workflow-custody/v1"}`},
				{"case_folded_duplicate", `{"workflow_id":"a","Workflow_ID":"b"}`},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					_, err := DecodeCustodyEnvelopeJSON([]byte(c.raw))
					require.ErrorIs(t, err, blerrors.ErrValidation)
				})
			}
		})

		t.Run("duplicate_yaml_keys_rejected", func(t *testing.T) {
			_, err := DecodeCustodyEnvelopeYAML([]byte("schema_version: workflow-custody/v1\nschema_version: workflow-custody/v1\n"))
			require.ErrorIs(t, err, blerrors.ErrValidation)
		})

		t.Run("json_and_yaml_payloads_round_trip", func(t *testing.T) {
			env := u3Envelope()
			raw, err := json.Marshal(env)
			require.NoError(t, err)
			decoded, err := DecodeCustodyEnvelopeJSON(raw)
			require.NoError(t, err)
			require.NoError(t, ValidateCustodyEnvelope(decoded))

			doc, err := yaml.Marshal(env)
			require.NoError(t, err)
			decodedYAML, err := DecodeCustodyEnvelopeYAML(doc)
			require.NoError(t, err)
			require.NoError(t, ValidateCustodyEnvelope(decodedYAML))
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
		t.Run("default_receipt_expiry_is_168_hours_after_acceptance", func(t *testing.T) {
			accepted := u3Clock()
			assert.Equal(t, DefaultCustodyReceiptExpiry, DefaultCustodyReceiptExpiresAt(accepted).Sub(accepted))
		})

		t.Run("request_identity_digest_binds_shipment", func(t *testing.T) {
			base := u3Envelope()
			other := u3Envelope()
			other.ShipmentID = "234-S"
			d1, err := CustodyRequestIdentityDigest(base)
			require.NoError(t, err)
			d2, err := CustodyRequestIdentityDigest(other)
			require.NoError(t, err)
			assert.NotEqual(t, d1, d2)
		})

		t.Run("mint_without_operator_signer_fails_closed", func(t *testing.T) {
			proof := u3ApprovalProof(t, u3Envelope())
			_, err := MintCustodyApprovalToken(proof)
			require.ErrorIs(t, err, ErrApprovalChannelUnavailable)
		})

		t.Run("mint_rejects_window_over_15_minutes", func(t *testing.T) {
			key, _ := u3OperatorFixture(t)
			proof := u3ApprovalProof(t, u3Envelope())
			proof.NotAfterUTC = proof.NotBeforeUTC.Add(MaxCustodyApprovalTokenWindow + time.Minute)
			_, err := MintCustodyApprovalTokenWithSigner(proof, key)
			require.ErrorIs(t, err, blerrors.ErrValidation)
		})

		t.Run("ed25519_mint_and_verify_round_trip", func(t *testing.T) {
			key, anchor := u3OperatorFixture(t)
			env := u3Envelope()
			token, err := MintCustodyApprovalTokenWithSigner(u3ApprovalProof(t, env), key)
			require.NoError(t, err)
			got, err := VerifyCustodyApprovalToken(token, env, u3Clock(), anchor)
			require.NoError(t, err)
			assert.Equal(t, "op-1", got.KeyID)
			assert.NotEmpty(t, got.Signature)
		})

		t.Run("verify_rejects_invalid_approvals_fail_closed", func(t *testing.T) {
			key, anchor := u3OperatorFixture(t)
			env := u3Envelope()
			mint := func(t *testing.T, proof CustodyApprovalProof) string {
				t.Helper()
				token, err := MintCustodyApprovalTokenWithSigner(proof, key)
				require.NoError(t, err)
				return token
			}
			cases := []struct {
				name    string
				proof   func(t *testing.T) CustodyApprovalProof
				now     time.Time
				mutate  func(*WorkflowCustodyEnvelope)
				anchors []config.TrustAnchor
				wantErr error
			}{
				{"wrong_role", func(t *testing.T) CustodyApprovalProof {
					p := u3ApprovalProof(t, env)
					p.AuthorizedRole = "stage"
					return p
				}, u3Clock(), nil, []config.TrustAnchor{anchor}, ErrOperatorApprovalRequired},
				{"expired_window", func(t *testing.T) CustodyApprovalProof { return u3ApprovalProof(t, env) }, u3Clock().Add(time.Hour), nil, []config.TrustAnchor{anchor}, ErrAuthorizationRejected},
				{"not_yet_valid", func(t *testing.T) CustodyApprovalProof { return u3ApprovalProof(t, env) }, u3Clock().Add(-time.Hour), nil, []config.TrustAnchor{anchor}, ErrAuthorizationRejected},
				{"wrong_workflow", func(t *testing.T) CustodyApprovalProof { return u3ApprovalProof(t, env) }, u3Clock(), func(e *WorkflowCustodyEnvelope) { e.WorkflowID = "wf-other" }, []config.TrustAnchor{anchor}, ErrAuthorizationRejected},
				{"wrong_shipment", func(t *testing.T) CustodyApprovalProof { return u3ApprovalProof(t, env) }, u3Clock(), func(e *WorkflowCustodyEnvelope) { e.ShipmentID = "234-S" }, []config.TrustAnchor{anchor}, ErrApprovalWrongShipment},
				{"wrong_scope_branch", func(t *testing.T) CustodyApprovalProof { return u3ApprovalProof(t, env) }, u3Clock(), func(e *WorkflowCustodyEnvelope) { e.ScopeBranch = "chore/other" }, []config.TrustAnchor{anchor}, ErrCustodyWrongBranch},
				{"stale_candidate_set", func(t *testing.T) CustodyApprovalProof { return u3ApprovalProof(t, env) }, u3Clock(), func(e *WorkflowCustodyEnvelope) { e.Receipts[0].CandidateSetDigest = "cand-other" }, []config.TrustAnchor{anchor}, ErrCustodyStaleDigest},
				{"changed_request_content", func(t *testing.T) CustodyApprovalProof { return u3ApprovalProof(t, env) }, u3Clock(), func(e *WorkflowCustodyEnvelope) { e.Receipts[0].Entries[0].Path = "docs/plans/other.md" }, []config.TrustAnchor{anchor}, ErrApprovalWrongRequestIdentity},
				{"revoked_anchor", func(t *testing.T) CustodyApprovalProof { return u3ApprovalProof(t, env) }, u3Clock(), nil, []config.TrustAnchor{revokedAnchor(anchor)}, ErrTrustAnchorRevoked},
				{"no_matching_anchor", func(t *testing.T) CustodyApprovalProof { return u3ApprovalProof(t, env) }, u3Clock(), nil, nil, ErrApprovalChannelUnavailable},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					token := mint(t, c.proof(t))
					verifyEnv := u3Envelope()
					if c.mutate != nil {
						c.mutate(&verifyEnv)
					}
					_, err := VerifyCustodyApprovalToken(token, verifyEnv, c.now, c.anchors...)
					require.ErrorIs(t, err, c.wantErr)
				})
			}
		})

		t.Run("verify_rejects_tampered_payload_without_resigning", func(t *testing.T) {
			key, anchor := u3OperatorFixture(t)
			env := u3Envelope()
			token, err := MintCustodyApprovalTokenWithSigner(u3ApprovalProof(t, env), key)
			require.NoError(t, err)
			parts := strings.Split(token, ".")
			raw, err := base64.RawURLEncoding.DecodeString(parts[1])
			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, json.Unmarshal(raw, &fields))
			fields["request_identity_digest"] = strings.Repeat("0", 64)
			payload, err := json.Marshal(fields)
			require.NoError(t, err)
			tampered := parts[0] + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + parts[2]
			_, err = VerifyCustodyApprovalToken(tampered, env, u3Clock(), anchor)
			require.ErrorIs(t, err, ErrAuthorizationRejected)
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

// TestU3_CaseOnlyRenameDirectoryEntryCasing checks that case_only_rename entries are
// bound to the on-disk directory-entry casing of their path (plan invariant I3).
// Only one file is created, because "plan.md" and "Plan.md" name the same file on
// case-insensitive filesystems.
func TestU3_CaseOnlyRenameDirectoryEntryCasing(t *testing.T) {
	newRoot := func(t *testing.T) *Workspace {
		t.Helper()
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "Plan.md"), []byte("placeholder\n"), 0o600))
		return &Workspace{RootPath: root}
	}

	t.Run("accepts_on_disk_casing_match", func(t *testing.T) {
		env := u3EnvelopeWithEntries(u3Rename("Plan.md", "plan.md", "6666", true))
		require.NoError(t, ValidateCustodyEnvelope(env))
		require.NoError(t, ValidateCustodyRenameCasing(newRoot(t), env))
	})

	t.Run("refuses_casing_mismatch_with_on_disk_entry", func(t *testing.T) {
		env := u3EnvelopeWithEntries(u3Rename("PLAN.md", "plan.md", "6666", true))
		err := ValidateCustodyRenameCasing(newRoot(t), env)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPartialRenameAdoptionRejected)
	})

	t.Run("refuses_missing_entry", func(t *testing.T) {
		env := u3EnvelopeWithEntries(u3Rename("MISSING.md", "missing.md", "6666", true))
		require.ErrorIs(t, ValidateCustodyRenameCasing(newRoot(t), env), ErrPartialRenameAdoptionRejected)
	})

	t.Run("refuses_missing_parent_directory", func(t *testing.T) {
		env := u3EnvelopeWithEntries(u3Rename("docs/Plan.md", "docs/plan.md", "6666", true))
		require.ErrorIs(t, ValidateCustodyRenameCasing(newRoot(t), env), ErrPartialRenameAdoptionRejected)
	})

	t.Run("refuses_parent_reached_through_link", func(t *testing.T) {
		ws := newRoot(t)
		u3Symlink(t, ws.RootPath, filepath.Join(ws.RootPath, "linkdir"))
		env := u3EnvelopeWithEntries(u3Rename("linkdir/Plan.md", "linkdir/plan.md", "6666", true))
		require.ErrorIs(t, ValidateCustodyRenameCasing(ws, env), ErrCustodyPathRejected)
	})

	t.Run("ignores_renames_that_are_not_case_only", func(t *testing.T) {
		env := u3EnvelopeWithEntries(u3Rename("docs/new.md", "docs/old.md", "6666", false))
		require.NoError(t, ValidateCustodyRenameCasing(newRoot(t), env))
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

// u3Clock is the fixed instant that every behavioral custody subtest uses.
func u3Clock() time.Time {
	return time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC)
}

// u3Entry returns a valid modified entry whose base, index, and worktree digests agree.
func u3Entry(path string) CustodyEntry {
	return CustodyEntry{
		Path: path, ChangeKind: "modified", StagedState: "unstaged_only",
		BaseBlobSHA256: "aaaa", IndexSHA256: "aaaa", WorktreeSHA256: "aaaa",
		PatchDigestSHA256: "pppp", Contributor: "stage", SourceKind: "stage_artifact",
	}
}

// u3Rename returns a renamed entry whose destination index and worktree digests agree.
func u3Rename(path, oldPath, oldBlobSHA string, caseOnly bool) CustodyEntry {
	return CustodyEntry{
		Path: path, OldPath: oldPath, ChangeKind: "renamed", CaseOnlyRename: caseOnly,
		StagedState: "staged_matches_worktree", OldBaseBlobSHA256: oldBlobSHA,
		IndexSHA256: "1111", WorktreeSHA256: "1111", PatchDigestSHA256: "pppp",
		Contributor: "operator", SourceKind: "operator_code_or_doc",
	}
}

// u3Envelope returns a valid offered workflow-custody/v1 envelope with one receipt.
func u3Envelope() WorkflowCustodyEnvelope {
	return WorkflowCustodyEnvelope{
		SchemaVersion: WorkflowCustodySchemaVersion,
		WorkflowID:    "wf-1",
		ShipmentID:    "233-S",
		ScopeBranch:   "chore/stage-233-S",
		BaseCommitSHA: "0123456789abcdef0123456789abcdef01234567",
		Receipts: []CustodyReceipt{{
			ReceiptID: "rcpt-0001", Revision: 1, Kind: "stage_to_ship_handoff",
			AuthorityProfile: "agent_workflow_handoff", Status: "offered",
			ProducerRole: "stage", AcceptingRole: "ship", AllowedAction: "stage_artifact_commit",
			CandidateSetDigest: "cand-1", OfferedAtUTC: u3Clock(),
			Entries: []CustodyEntry{u3Entry("docs/plans/2026-11-02-plan.md")},
		}},
	}
}

// u3EnvelopeWithEntries returns u3Envelope with its receipt's entries replaced.
func u3EnvelopeWithEntries(entries ...CustodyEntry) WorkflowCustodyEnvelope {
	env := u3Envelope()
	env.Receipts[0].Entries = entries
	return env
}

// u3OperatorFixture generates a fresh Ed25519 operator key and an inline-PEM trust anchor.
func u3OperatorFixture(t *testing.T) (ed25519.PrivateKey, config.TrustAnchor) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	pemText := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	return priv, config.TrustAnchor{
		ID: "op-1", Role: "operator", Algo: config.TrustAnchorAlgoEd25519,
		PublicKeyRef: string(pemText), Status: config.TrustAnchorStatusActive,
	}
}

// u3ApprovalProof returns an operator proof bound to env, valid for five minutes around u3Clock.
func u3ApprovalProof(t *testing.T, env WorkflowCustodyEnvelope) CustodyApprovalProof {
	t.Helper()
	digest, err := CustodyRequestIdentityDigest(env)
	require.NoError(t, err)
	receipt := env.Receipts[0]
	now := u3Clock()
	return CustodyApprovalProof{
		KeyID: "op-1", Algorithm: "ed25519", Principal: "operator:local", AuthorizedRole: "operator",
		WorkflowID: env.WorkflowID, ShipmentID: env.ShipmentID, ScopeBranch: env.ScopeBranch,
		BaseCommitSHA: env.BaseCommitSHA, CandidateSetDigest: receipt.CandidateSetDigest,
		AllowedAction: receipt.AllowedAction, TargetRevision: 1,
		NotBeforeUTC: now.Add(-time.Minute), NotAfterUTC: now.Add(5 * time.Minute),
		Nonce: "nonce-0001", RequestIdentityDigest: digest,
	}
}

// revokedAnchor returns a copy of anchor with revoked status.
func revokedAnchor(anchor config.TrustAnchor) config.TrustAnchor {
	anchor.Status = config.TrustAnchorStatusRevoked
	return anchor
}
