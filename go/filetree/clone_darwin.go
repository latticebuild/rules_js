package filetree

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func cloneFile(in *os.File, to string) error {
	var stat unix.Stat_t
	if err := unix.Fstat(int(in.Fd()), &stat); err != nil {
		return err
	}
	// clonefile also copies flags and extended attributes. Keep the byte-copy
	// path for these files, including compressed data and immutable/cache inputs.
	if stat.Flags != 0 {
		return errCloneUnsupported
	}
	// macOS also attaches provenance to ordinary new files. Accept only this
	// OS-managed entry; a bounded name list rejects all other source metadata.
	const provenance = "com.apple.provenance\x00"
	var attrs [len(provenance)]byte
	size, err := unix.Flistxattr(int(in.Fd()), attrs[:])
	if err == unix.ERANGE {
		return errCloneUnsupported
	}
	if err != nil {
		return cloneError(err)
	}
	if size != 0 && (size != len(attrs) || string(attrs[:]) != provenance) {
		return errCloneUnsupported
	}
	// No owner or source ACL inheritance; failed creation is atomic.
	if err := unix.Fclonefileat(int(in.Fd()), unix.AT_FDCWD, to, unix.CLONE_NOOWNERCOPY); err != nil {
		return cloneError(err)
	}
	// Ordinary copies have fresh timestamps, which incremental tools can observe.
	if err := unix.Utimes(to, nil); err != nil {
		if removeErr := os.Remove(to); removeErr != nil {
			return errors.Join(err, removeErr)
		}
		return err
	}
	return nil
}

func cloneError(err error) error {
	switch err {
	case unix.ENOTSUP, unix.EXDEV, unix.ENOSYS:
		return errCloneUnsupported
	default:
		return err
	}
}
