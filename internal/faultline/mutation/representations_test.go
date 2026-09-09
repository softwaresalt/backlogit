package mutation_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline/mutation"
)

// TestU1_RepresentationKindConstants verifies that all six expected
// RepresentationKind constants are exported with their correct string values.
func TestU1_RepresentationKindConstants(t *testing.T) {
	cases := []struct {
		name string
		kind mutation.RepresentationKind
		want string
	}{
		{"Frontmatter", mutation.Frontmatter, "frontmatter"},
		{"SQLite", mutation.SQLite, "sqlite"},
		{"EventsJSONL", mutation.EventsJSONL, "events_jsonl"},
		{"ArchiveFile", mutation.ArchiveFile, "archive_file"},
		{"ShipmentRecord", mutation.ShipmentRecord, "shipment_record"},
		{"CommitLink", mutation.CommitLink, "commit_link"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, mutation.RepresentationKind(tc.want), tc.kind)
		})
	}
}

// TestU1_RepresentationSetValidate verifies that a valid RepresentationSet
// passes schema validation without error.
func TestU1_RepresentationSetValidate(t *testing.T) {
	cases := []struct {
		name string
		set  mutation.RepresentationSet
	}{
		{
			name: "single_kind",
			set: mutation.RepresentationSet{
				Op:              "TestOp",
				Representations: []mutation.RepresentationKind{mutation.Frontmatter},
			},
		},
		{
			name: "all_kinds",
			set: mutation.RepresentationSet{
				Op: "FullOp",
				Representations: []mutation.RepresentationKind{
					mutation.Frontmatter,
					mutation.SQLite,
					mutation.EventsJSONL,
					mutation.ArchiveFile,
					mutation.ShipmentRecord,
					mutation.CommitLink,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.set.Validate())
		})
	}
}

// TestU1_RepresentationSetValidateErrors verifies that Validate returns the
// correct sentinel errors for each invalid input case.
func TestU1_RepresentationSetValidateErrors(t *testing.T) {
	cases := []struct {
		name    string
		set     mutation.RepresentationSet
		wantErr error
	}{
		{
			name:    "empty_op",
			set:     mutation.RepresentationSet{Op: "", Representations: []mutation.RepresentationKind{mutation.SQLite}},
			wantErr: mutation.ErrEmptyOp,
		},
		{
			name:    "nil_representations",
			set:     mutation.RepresentationSet{Op: "Op1", Representations: nil},
			wantErr: mutation.ErrEmptyRepresentations,
		},
		{
			name:    "empty_representations",
			set:     mutation.RepresentationSet{Op: "Op1", Representations: []mutation.RepresentationKind{}},
			wantErr: mutation.ErrEmptyRepresentations,
		},
		{
			name: "unknown_kind",
			set: mutation.RepresentationSet{
				Op:              "Op1",
				Representations: []mutation.RepresentationKind{"not_a_kind"},
			},
			wantErr: mutation.ErrUnknownKind,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.set.Validate()
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.wantErr), "expected errors.Is(%v) to be true, got: %v", tc.wantErr, err)
		})
	}
}

// TestU1_RegistryRegisterAndLookup verifies that Register stores a valid set
// and Lookup retrieves it by op name.
func TestU1_RegistryRegisterAndLookup(t *testing.T) {
	set := mutation.RepresentationSet{
		Op:              "LookupTestOp",
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite},
	}

	err := mutation.Register(set)
	require.NoError(t, err)

	got, ok := mutation.Lookup("LookupTestOp")
	require.True(t, ok, "Lookup should find registered op")
	assert.Equal(t, set.Op, got.Op)
	assert.Equal(t, set.Representations, got.Representations)
}

// TestU1_RegistryDuplicateReturnsError verifies that Register returns an error
// when the same op name is registered twice.
func TestU1_RegistryDuplicateReturnsError(t *testing.T) {
	set := mutation.RepresentationSet{
		Op:              "DuplicateTestOp",
		Representations: []mutation.RepresentationKind{mutation.EventsJSONL},
	}

	require.NoError(t, mutation.Register(set))

	err := mutation.Register(set)
	require.Error(t, err, "second registration should fail")
	assert.True(t, errors.Is(err, mutation.ErrDuplicateOp), "expected ErrDuplicateOp, got: %v", err)
}

// TestU1_RegistryRegistered verifies that Registered returns all registered op
// names in sorted order.
func TestU1_RegistryRegistered(t *testing.T) {
	names := mutation.Registered()
	require.NotEmpty(t, names, "Registered must return at least the declaration ops")

	// Verify sorted order.
	for i := 1; i < len(names); i++ {
		assert.Less(t, names[i-1], names[i], "Registered must return names in sorted order")
	}
}

// TestU1_DeclarationsAtLeastTwoOpsRegistered verifies that the declarations
// init registers at least two ops in the package-level registry.
func TestU1_DeclarationsAtLeastTwoOpsRegistered(t *testing.T) {
	names := mutation.Registered()
	assert.GreaterOrEqual(t, len(names), 2, "at least 2 ops must be declared via init")
}

// TestU1_DeclarationsSchemaValidates verifies that every declaration registered
// by init passes schema validation.
func TestU1_DeclarationsSchemaValidates(t *testing.T) {
	names := mutation.Registered()
	require.NotEmpty(t, names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			set, ok := mutation.Lookup(name)
			require.True(t, ok, "Lookup must succeed for registered op %q", name)
			require.NoError(t, set.Validate(), "declaration for %q must pass schema validation", name)
		})
	}

	// Explicitly check the two required ops are present with the expected kinds.
	t.Run("CreateItem_present", func(t *testing.T) {
		set, ok := mutation.Lookup("CreateItem")
		require.True(t, ok)
		assert.Contains(t, set.Representations, mutation.Frontmatter)
		assert.Contains(t, set.Representations, mutation.SQLite)
		assert.Contains(t, set.Representations, mutation.EventsJSONL)
	})

	t.Run("ArchiveItem_present", func(t *testing.T) {
		set, ok := mutation.Lookup("ArchiveItem")
		require.True(t, ok)
		assert.Contains(t, set.Representations, mutation.Frontmatter)
		assert.Contains(t, set.Representations, mutation.SQLite)
		assert.Contains(t, set.Representations, mutation.EventsJSONL)
		assert.Contains(t, set.Representations, mutation.ArchiveFile)
	})
}
