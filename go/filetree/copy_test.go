package filetree

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCopyFileIsolation(t *testing.T) {
	for _, size := range []int{0, 17, 256*1024 + 17} {
		for _, method := range []string{"native", "ordinary"} {
			t.Run(fmt.Sprintf("%s/%d", method, size), func(t *testing.T) {
				from, to := copyPaths(t)
				body := bytes.Repeat([]byte("x"), size)
				writeFile(t, from, body)
				old := time.Unix(1_600_000_000, 0)
				if err := os.Chtimes(from, old, old); err != nil {
					t.Fatal(err)
				}
				clone := cloneFile
				if method == "ordinary" {
					clone = noClone
				}
				if err := copyFile(from, to, clone); err != nil {
					t.Fatal(err)
				}
				assertContents(t, to, body)
				source, err := os.Stat(from)
				if err != nil {
					t.Fatal(err)
				}
				destination, err := os.Stat(to)
				if err != nil {
					t.Fatal(err)
				}
				if os.SameFile(source, destination) || !destination.ModTime().After(old.Add(24*time.Hour)) {
					t.Fatalf("copy aliases its source or retained its timestamp: %v", destination)
				}
				writeFile(t, to, []byte("destination edit"))
				assertContents(t, from, body)
				writeFile(t, from, []byte("source edit"))
				assertContents(t, to, []byte("destination edit"))
			})
		}
	}
}

func TestNativeClone(t *testing.T) {
	from, to := copyPaths(t)
	body := append(bytes.Repeat([]byte("native clone\n"), 32*1024), []byte("tail")...)
	writeFile(t, from, body)
	in, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := in.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := cloneFile(in, to); err == errCloneUnsupported {
		if _, err := os.Stat(to); !os.IsNotExist(err) {
			t.Fatalf("unsupported clone left destination: %v", err)
		}
		t.Skip("native cloning unavailable on this test filesystem; fallback is tested separately")
	} else if err != nil {
		t.Fatal(err)
	}
	assertContents(t, to, body)
	overwriteByte(t, to, 1, 'D')
	assertContents(t, from, body)
	overwriteByte(t, from, 2, 'S')
	body[1] = 'D'
	assertContents(t, to, body)
}

func TestCopyFallbackAfterPartialClone(t *testing.T) {
	from, to := copyPaths(t)
	body := []byte("complete original bytes")
	writeFile(t, from, body)
	err := copyFile(from, to, func(in *os.File, to string) error {
		return writeCopy(to, func(out *os.File) error {
			_, err := cloneRanges(2<<30, 4096, func(offset, _ int64) error {
				if offset != 0 {
					return errCloneUnsupported
				}

				_, err := io.CopyN(out, in, 8)
				return err
			})
			return err
		})
	})

	if err != nil {
		t.Fatal(err)
	}
	assertContents(t, to, body)
	assertContents(t, from, body)
}

func TestCopyPropagatesCloneFailure(t *testing.T) {
	from, to := copyPaths(t)
	writeFile(t, from, []byte("input"))
	err := copyFile(from, to, func(_ *os.File, to string) error {
		return writeCopy(to, func(out *os.File) error {
			if _, err := out.WriteString("partial"); err != nil {
				return err
			}
			return io.ErrUnexpectedEOF
		})
	})

	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("copy error = %v", err)
	}
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		t.Fatalf("partial destination remains: %v", err)
	}
}

func TestCopyDoesNotHideCleanupFailure(t *testing.T) {
	from, to := copyPaths(t)
	writeFile(t, from, []byte("input"))
	err := copyFile(from, to, func(_ *os.File, to string) error {
		return writeCopy(to, func(out *os.File) error {

			if err := out.Close(); err != nil {
				return err
			}
			if err := os.Remove(to); err != nil {
				return err
			}
			if err := os.Mkdir(to, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(to, "keep"), []byte("retained"), 0o600); err != nil {
				return err
			}
			return errCloneUnsupported
		})
	})

	if !errors.Is(err, errCloneUnsupported) || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("lost clone/close failure: %v", err)
	}
	assertContents(t, filepath.Join(to, "keep"), []byte("retained"))
}

func TestCopyRefusesExistingDestination(t *testing.T) {
	for _, clone := range []func(*os.File, string) error{cloneFile, noClone} {
		from, to := copyPaths(t)
		writeFile(t, from, []byte("input"))
		writeFile(t, to, []byte("existing"))
		if err := copyFile(from, to, clone); !os.IsExist(err) && !errors.Is(err, os.ErrExist) {
			t.Fatalf("existing destination error = %v", err)
		}
		assertContents(t, to, []byte("existing"))
	}
}

func TestCloneRanges(t *testing.T) {
	for _, cluster := range []int64{4096, 65536} {
		var got [][2]int64
		size := int64(5<<30) + cluster + 17
		tail, err := cloneRanges(size, cluster, func(offset, length int64) error {
			got = append(got, [2]int64{offset, length})
			return nil
		})
		want := [][2]int64{{0, 1 << 30}, {1 << 30, 1 << 30}, {2 << 30, 1 << 30}, {3 << 30, 1 << 30}, {4 << 30, 1 << 30}, {5 << 30, cluster}}
		if err != nil || tail != size-17 || !reflect.DeepEqual(got, want) {
			t.Fatalf("ranges(%d) = %v, %d, %v", cluster, got, tail, err)
		}
	}
	for _, size := range []int64{0, 17} {
		tail, err := cloneRanges(size, 4096, func(_, _ int64) error {
			t.Fatal("cloned a partial cluster")
			return nil
		})
		if tail != 0 || err != nil {
			t.Fatalf("small file = %d, %v", tail, err)
		}
	}
	for _, cluster := range []int64{0, -1, 4095, 2 << 30} {
		if _, err := cloneRanges(8192, cluster, func(_, _ int64) error { return nil }); err == nil {
			t.Fatalf("accepted invalid cluster size %d", cluster)
		}
	}
}

func BenchmarkCopyFiles(b *testing.B) {
	for _, workload := range []struct {
		name        string
		count, size int
	}{{"small", 1000, 4096}, {"large", 4, 16 << 20}} {
		b.Run(workload.name, func(b *testing.B) {
			root := b.TempDir()
			body := bytes.Repeat([]byte("x"), workload.size)
			for i := range workload.count {
				writeFile(b, filepath.Join(root, fmt.Sprintf("source-%d", i)), body)
			}
			for _, method := range []string{"native", "ordinary"} {
				b.Run(method, func(b *testing.B) {
					clone := cloneFile
					if method == "ordinary" {
						clone = noClone
					}
					b.SetBytes(int64(workload.count * workload.size))
					for b.Loop() {
						for i := range workload.count {
							to := filepath.Join(root, "destination")
							if err := copyFile(filepath.Join(root, fmt.Sprintf("source-%d", i)), to, clone); err != nil {
								b.Fatal(err)
							}
							if err := os.Remove(to); err != nil {
								b.Fatal(err)
							}
						}
					}
				})
			}
		})
	}
}

func copyPaths(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	return filepath.Join(root, "source"), filepath.Join(root, "destination")
}

func noClone(_ *os.File, _ string) error {
	return errCloneUnsupported
}

func writeFile(t testing.TB, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertContents(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s contents differ: got %d bytes, want %d", path, len(got), len(want))
	}
}

func overwriteByte(t *testing.T, path string, offset int64, value byte) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteAt([]byte{value}, offset)
	if err := errors.Join(err, file.Close()); err != nil {
		t.Fatal(err)
	}
}
