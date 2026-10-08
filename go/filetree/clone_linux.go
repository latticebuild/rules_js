package filetree

import (
	"os"

	"golang.org/x/sys/unix"
)

func cloneFile(in *os.File, to string) error {
	return writeCopy(to, func(out *os.File) error {
		return cloneError(unix.IoctlFileClone(int(out.Fd()), int(in.Fd())))
	})
}

func cloneError(err error) error {
	switch err {
	// EINVAL includes filesystems whose file layout cannot be reflinked.
	case unix.EOPNOTSUPP, unix.ENOTTY, unix.EXDEV, unix.ENOSYS, unix.EINVAL:
		return errCloneUnsupported
	default:
		return err
	}
}
