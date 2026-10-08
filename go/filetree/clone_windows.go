package filetree

import (
	"fmt"
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// FSCTL_GET_INTEGRITY_INFORMATION_BUFFER from winioctl.h.
type integrityInformation struct {
	checksum  uint16
	reserved  uint16
	flags     uint32
	chunkSize uint32
	cluster   uint32
}

// DUPLICATE_EXTENTS_DATA aligns its LARGE_INTEGER fields to eight bytes.
// The uint64 handle slot also supplies the padding on 32-bit Windows.
type duplicateExtents struct {
	source       uint64
	sourceOffset int64
	targetOffset int64
	length       int64
}

func cloneFile(in *os.File, to string) error {
	var info integrityInformation
	var returned uint32
	err := windows.DeviceIoControl(windows.Handle(in.Fd()), windows.FSCTL_GET_INTEGRITY_INFORMATION,
		nil, 0, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), &returned, nil)
	if err != nil {
		return cloneError(err)
	}
	if returned != uint32(unsafe.Sizeof(info)) || info.cluster == 0 || info.cluster > 1<<30 || info.cluster&(info.cluster-1) != 0 {
		return fmt.Errorf("invalid clone integrity information: %d bytes, cluster size %d", returned, info.cluster)
	}
	stat, err := in.Stat()
	if err != nil {
		return err
	}
	if stat.Size() < int64(info.cluster) {
		return errCloneUnsupported
	}
	return writeCopy(to, func(out *os.File) error {
		// ReFS cannot clone beyond the destination's current end of file.
		if err := out.Truncate(stat.Size()); err != nil {
			return err
		}
		tail, err := cloneRanges(stat.Size(), int64(info.cluster), func(offset, length int64) error {
			data := duplicateExtents{
				source: uint64(in.Fd()), sourceOffset: offset, targetOffset: offset, length: length,
			}
			return cloneError(windows.DeviceIoControl(windows.Handle(out.Fd()), windows.FSCTL_DUPLICATE_EXTENTS_TO_FILE,
				(*byte)(unsafe.Pointer(&data)), uint32(unsafe.Sizeof(data)), nil, 0, &returned, nil))
		})
		if err != nil {
			return err
		}
		if tail != stat.Size() {
			if _, err := out.Seek(tail, io.SeekStart); err != nil {
				return err
			}
			_, err = io.CopyN(out, io.NewSectionReader(in, tail, stat.Size()-tail), stat.Size()-tail)
		}
		return err
	})
}

func cloneError(err error) error {
	switch err {
	// INVALID_PARAMETER also covers sparse/integrity settings that cannot share
	// extents. Preserve the source and use ordinary bytes for those files.
	case windows.ERROR_INVALID_FUNCTION, windows.ERROR_NOT_SUPPORTED, windows.ERROR_NOT_SAME_DEVICE, windows.ERROR_INVALID_PARAMETER:
		return errCloneUnsupported
	default:
		return err
	}
}
