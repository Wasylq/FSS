//go:build windows

package store

import "os"

// lockFile on Windows opens the file but does not acquire an advisory
// lock. Concurrent scrapes of the same studio URL may race on Windows.
func lockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
}
