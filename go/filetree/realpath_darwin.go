package filetree

import (
	"golang.org/x/sys/unix"
	"os"
	"unsafe"
)

// RealPath resolves file and directory aliases without opening file contents.
func RealPath(path string) (string, error) {
	return resolveMetadataPath(path, func(path string) (*os.File, error) {
		fd, err := unix.Open(path, unix.O_EVTONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			return nil, err
		}
		return os.NewFile(uintptr(fd), path), nil
	}, func(file *os.File) (string, error) {
		// Darwin PATH_MAX includes the terminating NUL.
		var buffer [1024]byte
		_, _, errno := unix.Syscall(unix.SYS_FCNTL, file.Fd(), unix.F_GETPATH, uintptr(unsafe.Pointer(&buffer[0])))
		if errno != 0 {
			return "", errno
		}
		return unix.ByteSliceToString(buffer[:]), nil
	}, true)
}
