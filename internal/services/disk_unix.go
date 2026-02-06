//go:build !windows

package services

import "syscall"

// GetDiskUsageBytes returns the number of bytes used on the filesystem containing the given path.
// This is exported for use by other services (e.g. disk limit checks).
func GetDiskUsageBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	return total - free, nil
}
