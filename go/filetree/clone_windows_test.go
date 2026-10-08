package filetree

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestCloneWindowsABI(t *testing.T) {
	var info integrityInformation
	var data duplicateExtents
	if unsafe.Sizeof(info) != 16 || unsafe.Offsetof(info.cluster) != 12 ||
		unsafe.Sizeof(data) != 32 || unsafe.Offsetof(data.sourceOffset) != 8 ||
		unsafe.Offsetof(data.targetOffset) != 16 || unsafe.Offsetof(data.length) != 24 {
		t.Fatal("block clone buffers do not match winioctl.h")
	}
}

func TestCloneErrors(t *testing.T) {
	for _, err := range []error{windows.ERROR_INVALID_FUNCTION, windows.ERROR_NOT_SUPPORTED, windows.ERROR_NOT_SAME_DEVICE, windows.ERROR_INVALID_PARAMETER} {
		if got := cloneError(err); got != errCloneUnsupported {
			t.Errorf("capability error %v became %v", err, got)
		}
	}
	for _, err := range []error{nil, windows.ERROR_ACCESS_DENIED, windows.ERROR_DISK_FULL, windows.ERROR_IO_DEVICE, windows.ERROR_FILE_EXISTS} {
		if got := cloneError(err); got != err {
			t.Errorf("fatal error %v became %v", err, got)
		}
	}
}
