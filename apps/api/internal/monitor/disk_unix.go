//go:build unix

package monitor

import "syscall"

// diskUsage reports the total and free bytes of the filesystem holding path.
func diskUsage(path string) (total, free uint64, ok bool) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return 0, 0, false
	}
	return fs.Blocks * uint64(fs.Bsize), fs.Bavail * uint64(fs.Bsize), true
}
