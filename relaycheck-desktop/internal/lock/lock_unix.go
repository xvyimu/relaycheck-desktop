//go:build !windows

package lock

import (
	"os"
	"syscall"
)

// Acquire opens (or creates) the file at path and tries to acquire an
// exclusive file lock. It returns the open *os.File on success; the caller
// must Close it (or defer close) to release the lock.
//
// If another process already holds the lock, ErrAlreadyLocked is returned.
func Acquire(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			return nil, ErrAlreadyLocked
		}
		return nil, err
	}
	return f, nil
}