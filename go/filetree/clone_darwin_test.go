package filetree

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCloneFallsBackForDarwinMetadata(t *testing.T) {
	for _, metadata := range []string{"attribute", "immutable"} {
		t.Run(metadata, func(t *testing.T) {
			from, to := copyPaths(t)
			writeFile(t, from, []byte("input"))
			if metadata == "attribute" {
				if err := unix.Setxattr(from, "user.latticebuild-test", []byte("metadata"), 0); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := unix.Chflags(from, unix.UF_IMMUTABLE); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := unix.Chflags(from, 0); err != nil {
						t.Error(err)
					}
				})
			}
			if err := copyFile(from, to, func(in *os.File, to string) error {
				err := cloneFile(in, to)
				if err != errCloneUnsupported {
					t.Errorf("metadata clone = %v; want ordinary-copy fallback", err)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			writeFile(t, to, []byte("writable"))
			assertContents(t, from, []byte("input"))
			var stat unix.Stat_t
			if err := unix.Stat(to, &stat); err != nil {
				t.Fatal(err)
			}
			_, err := unix.Getxattr(to, "user.latticebuild-test", nil)
			if err != unix.ENOATTR || stat.Flags != 0 {
				t.Fatalf("copied source metadata: flags=%d, custom attribute error=%v", stat.Flags, err)
			}
		})
	}
}
