package filetree

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Go classifies junctions as irregular files, so EvalSymlinks cannot walk
// through them. Node reports the final directory path in its coverage output.
// RealPath resolves file and directory aliases, including Windows junctions.
func RealPath(directory string) (string, error) {
	name, err := windows.UTF16PtrFromString(windowsPath(directory))
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(name, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 256)
	for {
		size, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
		if err != nil {
			return "", err
		}
		if size >= uint32(len(buffer)) {
			buffer = make([]uint16, size+1)
			continue
		}
		real := windows.UTF16ToString(buffer[:size])
		if strings.HasPrefix(real, `\\?\UNC\`) {
			real = `\\` + strings.TrimPrefix(real, `\\?\UNC\`)
		} else if strings.HasPrefix(real, `\\?\`) && len(real) >= 6 && real[5] == ':' {
			real = real[4:]
		}
		return filepath.Clean(real), nil
	}
}

func windowsPath(name string) string {
	if strings.HasPrefix(name, `\\?\`) || strings.HasPrefix(name, `\\.\`) {
		return name
	}
	if strings.HasPrefix(name, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(name, `\\`)
	}
	if filepath.IsAbs(name) {
		return `\\?\` + filepath.Clean(name)
	}
	return name
}
