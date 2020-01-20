package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hashicorp/go-getter"
)

// GetMode is an enum that describes how modules are loaded.
//
// GetModeLoad says that modules will not be downloaded or updated, they will
// only be loaded from the storage.
//
// GetModeGet says that modules can be initially downloaded if they don't
// exist, but otherwise to just load from the current version in storage.
//
// GetModeUpdate says that modules should be checked for updates and
// downloaded prior to loading. If there are no updates, we load the version
// from disk, otherwise we download first and then load.
type GetMode byte

const (
	GetModeNone GetMode = iota
	GetModeGet
	GetModeUpdate
)

// GetCopy is the same as Get except that it downloads a copy of the
// module represented by source.
//
// This copy will omit and dot-prefixed files (such as .git/, .hg/) and
// can't be updated on its own.
func GetCopy(dst, src string) error {
	// Create the temporary directory to do the real Get to
	tmpDir, err := ioutil.TempDir("", "tf")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpDir = filepath.Join(tmpDir, "module")

	// Get to that temporary dir
	if err := getter.Get(tmpDir, src); err != nil {
		return err
	}

	// Make sure the destination exists
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// Copy to the final location
	return copyDir(dst, tmpDir)
}

// copyDir copies the src directory contents into dst. Both directories
// should already exist.
func copyDir(dst, src string) error {
	src, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == src {
			return nil
		}

		if strings.HasPrefix(filepath.Base(path), ".") {
			// Skip any dot files
			if info.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		// The "path" has the src prefixed to it. We need to join our
		// destination with the path without the src on it.
		dstPath := filepath.Join(dst, path[len(src):])

		// we don't want to try and copy the same file over itself.
		if eq, err := sameFile(path, dstPath); eq {
			return nil
		} else if err != nil {
			return err
		}

		// If we have a directory, make that subdirectory, then continue
		// the walk.
		if info.IsDir() {
			if path == filepath.Join(src, dst) {
				// dst is in src; don't walk it.
				return nil
			}

			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}

			return nil
		}

		// If we have a file, copy the contents.
		srcF, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcF.Close()

		dstF, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstF.Close()

		if _, err := io.Copy(dstF, srcF); err != nil {
			return err
		}

		// Chmod it
		return os.Chmod(dstPath, info.Mode())
	}

	return filepath.Walk(src, walkFn)
}

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
