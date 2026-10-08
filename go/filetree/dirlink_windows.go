package filetree

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func dirLink(target, link string) error {
	// Junctions support local volumes. In particular, a mapped network drive is
	// not a valid substitute for a local directory, even if it has a drive letter.
	volume := filepath.VolumeName(target)
	if strings.HasPrefix(volume, `\\`) && !strings.HasPrefix(volume, `\\?\`) {
		return fmt.Errorf("directory junction target %s is on a remote share", target)
	}
	drive, err := windows.UTF16PtrFromString(volume + `\`)
	if err != nil {
		return err
	}
	if windows.GetDriveType(drive) == windows.DRIVE_REMOTE || strings.HasPrefix(target, `\\?\UNC\`) {
		return fmt.Errorf("directory junction target %s is on a remote share", target)
	}
	substitute, err := windows.UTF16FromString(`\??\` + strings.TrimPrefix(target, `\\?\`))
	if err != nil {
		return err
	}
	printed, err := windows.UTF16FromString(target)
	if err != nil {
		return err
	}
	// REPARSE_DATA_BUFFER: 8-byte header, four 16-bit mount-point fields,
	// then null-terminated substitute and print names. Lengths exclude nulls.
	buffer := make([]byte, 16+2*(len(substitute)+len(printed)))
	if len(buffer) > windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE {
		return fmt.Errorf("directory junction target %s exceeds the reparse-point limit", target)
	}
	binary.LittleEndian.PutUint32(buffer[0:4], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buffer[4:6], uint16(len(buffer)-8))
	binary.LittleEndian.PutUint16(buffer[10:12], uint16(2*(len(substitute)-1)))
	binary.LittleEndian.PutUint16(buffer[12:14], uint16(2*len(substitute)))
	binary.LittleEndian.PutUint16(buffer[14:16], uint16(2*(len(printed)-1)))
	for i, unit := range append(substitute, printed...) {
		binary.LittleEndian.PutUint16(buffer[16+2*i:18+2*i], unit)
	}
	return junction(link, buffer)
}

func junction(link string, buffer []byte) error {
	if err := os.Mkdir(windowsPath(link), 0o700); err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(windowsPath(link))
		}
	}()
	name, err := windows.UTF16PtrFromString(windowsPath(link))
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var returned uint32
	if len(buffer) == 0 {
		return fmt.Errorf("empty directory junction data")
	}
	if err := windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &buffer[0], uint32(len(buffer)), nil, 0, &returned, nil); err != nil {
		return err
	}
	complete = true
	return nil
}
