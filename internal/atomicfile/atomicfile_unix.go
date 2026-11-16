//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package atomicfile

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// atomicReplace renames tmpName over path with a plain POSIX os.Rename, which
// replaces the destination atomically without ever removing it first. The
// durable argument is unused here: on POSIX the rename's own durability is
// provided by the parent-directory fsync in WriteFileAtomicWithOptions (durable
// mode), not by a per-rename flag. On failure the destination is untouched, so
// the error is classified ErrWriteNotApplied.
func atomicReplace(tmpName, path string, durable bool) error {
	_ = durable // POSIX rename needs no durability flag; see doc above.
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp over destination: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, err))
	}
	return nil
}

func verifyUnixRootLeaf(dirFD int, rel string, expected os.FileInfo) error {
	var st unix.Stat_t
	if err := unix.Fstatat(dirFD, rel, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("fstatat %q: %w", rel, err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("%q is not a regular file: %w", rel, blerrors.ErrValidation)
	}
	if sysStat, ok := expected.Sys().(*syscall.Stat_t); ok {
		if uint64(st.Dev) != uint64(sysStat.Dev) || uint64(st.Ino) != uint64(sysStat.Ino) {
			return fmt.Errorf("%q inode changed before renameat: %w", rel, blerrors.ErrValidation)
		}
	}
	return nil
}

func replaceTempFileInRootPlatform(
	_ *os.Root,
	dirFile *os.File,
	dirAbs string,
	tmpRel, dstRel string,
	tmpInfo, dstInfo os.FileInfo,
	durable bool,
) error {
	dirFileStat, err := dirFile.Stat()
	if err != nil {
		return notApplied("stat pinned parent dir handle", err)
	}
	preDirLstat, err := os.Lstat(dirAbs)
	if err != nil {
		return notApplied("lstat parent dir before renameat", err)
	}
	if preDirLstat.Mode()&os.ModeSymlink != 0 || !preDirLstat.IsDir() || !os.SameFile(preDirLstat, dirFileStat) {
		return notApplied("parent dir changed or redirected before renameat", blerrors.ErrValidation)
	}

	dirFD := int(dirFile.Fd())
	if err := verifyUnixRootLeaf(dirFD, tmpRel, tmpInfo); err != nil {
		return notApplied("verify temp leaf before renameat", err)
	}
	if dstInfo != nil {
		if err := verifyUnixRootLeaf(dirFD, dstRel, dstInfo); err != nil {
			return notApplied("verify destination leaf before renameat", err)
		}
	}

	if err := unix.Renameat(dirFD, tmpRel, dirFD, dstRel); err != nil {
		return notApplied("renameat temp over destination", err)
	}
	postDirLstat, postErr := os.Lstat(dirAbs)
	if postErr != nil {
		return fmt.Errorf("lstat parent dir after renameat: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteIndeterminate, postErr))
	}
	if postDirLstat.Mode()&os.ModeSymlink != 0 || !postDirLstat.IsDir() || !os.SameFile(postDirLstat, dirFileStat) {
		return fmt.Errorf("parent dir changed during renameat: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteIndeterminate, blerrors.ErrValidation))
	}
	if durable {
		if err := dirFile.Sync(); err != nil {
			return fmt.Errorf("fsync pinned parent dir after renameat: %w",
				fmt.Errorf("%w: %w", blerrors.ErrWriteIndeterminate, err))
		}
	}
	return nil
}
