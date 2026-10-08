//go:build darwin || linux

package filetree

import (
	"runtime"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCloneErrors(t *testing.T) {
	for _, err := range []error{unix.ENOTSUP, unix.EXDEV, unix.ENOSYS} {
		if got := cloneError(err); got != errCloneUnsupported {
			t.Errorf("capability error %v became %v", err, got)
		}
	}
	for _, err := range []error{nil, unix.EACCES, unix.EPERM, unix.ENOSPC, unix.EIO, unix.EEXIST} {
		if got := cloneError(err); got != err {
			t.Errorf("fatal error %v became %v", err, got)
		}
	}
	for _, err := range []error{unix.EINVAL, unix.ENOTTY} {
		want := err
		if runtime.GOOS == "linux" {
			want = errCloneUnsupported
		}
		if got := cloneError(err); got != want {
			t.Errorf("layout error %v became %v; want %v", err, got, want)
		}
	}
}
