// +build windows

package util

// sameFile tried to determine if to paths are the same file.
// If the paths don't match, we lookup the inode on supported systems.
func sameFile(a, b string) (bool, error) {
	return a == b, nil
}
