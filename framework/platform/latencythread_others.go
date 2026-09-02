//go:build !windows

package platform

// BeginLatencySensitiveThread is a no-op on platforms without Windows MMCSS.
func BeginLatencySensitiveThread() (func() error, error) {
	return func() error { return nil }, nil
}
