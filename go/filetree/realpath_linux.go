package filetree

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// RealPath resolves file and directory aliases using a metadata-only handle.
func RealPath(path string) (string, error) {
	return resolveMetadataPath(path, func(path string) (*os.File, error) {
		fd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
		if err != nil {
			return nil, err
		}
		return os.NewFile(uintptr(fd), path), nil
	}, func(file *os.File) (string, error) {
		return os.Readlink(fmt.Sprintf("/proc/self/fd/%d", file.Fd()))
	}, false)
}
