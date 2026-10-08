package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/latticebuild/rules_js/go/filetree"

	// Compare checks that each pair an inventory lists holds the same package
	// output in the target and execution configurations, then writes stamp. A
	// Vite build lets the target copy stand in for the execution copy only after
	// this succeeds, so the stamp is written only when every pair matches. The
	// inventory is JSON [[name, target, execution], ...] with execroot paths.
	"io"
	"os"
	"path/filepath"
)

func compare(execroot, inventory, stamp string) error {
	list, err := filetree.Path(execroot, inventory)
	if err != nil {
		return err
	}
	out, err := filetree.Path(execroot, stamp)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(list)
	if err != nil {
		return err
	}
	var pairs [][3]string
	if err := json.Unmarshal(data, &pairs); err != nil {
		return fmt.Errorf("inventory %s: %w", inventory, err)
	}
	for _, pair := range pairs {
		name, target, execution := pair[0], pair[1], pair[2]
		same, err := sameOutput(execroot, target, execution)
		if err != nil {
			return fmt.Errorf("package output %s: %w", name, err)
		}
		if !same {
			return fmt.Errorf("conflicting target and execution package output %s: %s and %s", name, target, execution)
		}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, []byte("equivalent\n"), 0o644)
}

// sameOutput reports whether two regular files, found through any symlinks,
// have the same size, permission bits and bytes.
func sameOutput(execroot, target, execution string) (bool, error) {
	left, err := filetree.Path(execroot, target)
	if err != nil {
		return false, err
	}
	right, err := filetree.Path(execroot, execution)
	if err != nil {
		return false, err
	}
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, err
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false, err
	}
	if !leftInfo.Mode().IsRegular() || !rightInfo.Mode().IsRegular() ||
		leftInfo.Size() != rightInfo.Size() || leftInfo.Mode().Perm() != rightInfo.Mode().Perm() {
		return false, nil
	}
	return sameBytes(left, right)
}

// sameBytes compares two files of equal size a chunk at a time.
func sameBytes(left, right string) (same bool, err error) {
	a, err := os.Open(left)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, a.Close()) }()
	b, err := os.Open(right)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, b.Close()) }()
	bufA, bufB := make([]byte, 64<<10), make([]byte, 64<<10)
	for {
		n, errA := io.ReadFull(a, bufA)
		m, errB := io.ReadFull(b, bufB)
		if n != m || !bytes.Equal(bufA[:n], bufB[:m]) {
			return false, nil
		}
		endA := errors.Is(errA, io.EOF) || errors.Is(errA, io.ErrUnexpectedEOF)
		endB := errors.Is(errB, io.EOF) || errors.Is(errB, io.ErrUnexpectedEOF)
		if errA != nil && !endA {
			return false, errA
		}
		if errB != nil && !endB {
			return false, errB
		}
		if endA || endB {
			return endA == endB, nil
		}
	}
}
