//go:build !darwin && !linux && !windows

package filetree

import "os"

func cloneFile(_ *os.File, _ string) error {
	return errCloneUnsupported
}
