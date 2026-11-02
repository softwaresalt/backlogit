package db

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReviewSnapshotWALSaltTailWithLaterCommit(t *testing.T) {
	_, path := reviewReadOnlyWriter(t)
	base, err := os.ReadFile(path)
	require.NoError(t, err)
	wal, err := os.ReadFile(path + "-wal")
	require.NoError(t, err)
	prefix, err := committedSnapshotWAL(base, wal)
	require.NoError(t, err)
	require.Equal(t, wal, prefix, "the starting WAL must be fully committed and verified")
	frameSize := 24 + int(binary.BigEndian.Uint32(wal[8:12]))
	lastFrame := wal[len(wal)-frameSize:]
	for _, test := range []struct {
		name            string
		checksumInvalid bool
		laterCommit     bool
	}{
		{name: "salt-corrupted-then-current-commit", laterCommit: true},
		{name: "checksum-invalid-then-salt-corrupted-then-current-commit", checksumInvalid: true, laterCommit: true},
		{name: "old-generation-tail"},
		{name: "checksum-invalid-then-old-generation-tail", checksumInvalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			image := append([]byte(nil), wal...)
			if test.checksumInvalid {
				uncommitted := append([]byte(nil), lastFrame...)
				binary.BigEndian.PutUint32(uncommitted[4:8], 0)
				uncommitted[24] ^= 0xff
				image = append(image, uncommitted...)
			}
			otherGeneration := append([]byte(nil), lastFrame...)
			otherGeneration[8] ^= 0xff
			// Multiple mismatches, including old-generation commit markers,
			// must not hide current-generation evidence farther into the tail.
			image = append(image, otherGeneration...)
			image = append(image, otherGeneration...)
			if test.laterCommit {
				image = append(image, lastFrame...)
			}
			valid, err := committedSnapshotWAL(base, image)
			if test.laterCommit {
				require.ErrorIs(t, err, errSnapshotChanged, "salt mismatch must not hide a later current-generation commit")
				require.Nil(t, valid, "ambiguous corruption must not return a stale committed prefix")
			} else {
				require.NoError(t, err, "old-generation commit markers must not prevent rollback/restart tail acceptance")
				require.Equal(t, prefix, valid)
			}
		})
	}
}
