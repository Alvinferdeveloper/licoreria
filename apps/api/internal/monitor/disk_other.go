//go:build !unix

package monitor

// diskUsage is only measured on the Linux server; elsewhere the check reports it as unknown.
func diskUsage(string) (total, free uint64, ok bool) { return 0, 0, false }
