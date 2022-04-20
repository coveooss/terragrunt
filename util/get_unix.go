//go:build linux || darwin
// +build linux darwin

package util

import (
	"fmt"
	"os"
	"syscall"
)

// sameFile tried to determine if to paths are the same file.
// If the paths don't match, we lookup the inode on supported systems.
func sameFile(a, b string) (bool, error) {
	if a == b {
		return true, nil
	}

	aIno, err := inode(a)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	bIno, err := inode(b)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	if aIno > 0 && aIno == bIno {
		return true, nil
	}

	return false, nil
}

// lookup the inode of a file on posix systems
func inode(path string) (uint64, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if st, ok := stat.Sys().(*syscall.Stat_t); ok {
		return st.Ino, nil
	}
	return 0, fmt.Errorf("could not determine file inode")
}
