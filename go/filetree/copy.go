package filetree

import (
	"errors"
	"fmt"
	"io"
	"os"
)

var errCloneUnsupported = errors.New("file cloning is unsupported")

// The native operation returns errCloneUnsupported only with no destination
// left behind. Each call supplies its operation so failure tests need no globals.
func copyFile(from, to string, clone func(*os.File, string) error) (err error) {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	err = clone(in, to)
	if err == nil {
		return nil
	}
	// A joined close/cleanup error must never turn into a successful fallback.
	if err != errCloneUnsupported {
		return fmt.Errorf("clone %s to %s: %w", from, to, err)
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return writeCopy(to, func(out *os.File) error {
		_, err := io.Copy(out, in)
		return err
	})
}

// Own only a newly created file. In particular, never truncate an existing
// destination or leave a partly cloned file for the ordinary-copy fallback.
func writeCopy(to string, write func(*os.File) error) error {
	out, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	err = write(out)
	if closeErr := out.Close(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	if err != nil {
		if removeErr := os.Remove(to); removeErr != nil {
			err = errors.Join(err, removeErr)
		}
	}
	return err
}

// ReFS requires cluster-aligned ranges smaller than 4 GiB. Return the offset
// of the remaining partial cluster, which the caller copies as ordinary bytes.
func cloneRanges(size, cluster int64, clone func(offset, length int64) error) (int64, error) {
	const chunk = int64(1 << 30)
	if size < 0 || cluster <= 0 || cluster > chunk || cluster&(cluster-1) != 0 {
		return 0, fmt.Errorf("invalid clone size %d or cluster size %d", size, cluster)
	}
	end := size - size%cluster
	for offset := int64(0); offset < end; {
		length := min(chunk, end-offset)
		if err := clone(offset, length); err != nil {
			return offset, err
		}
		offset += length
	}
	return end, nil
}
