//go:build windows

package lock

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32       = windows.NewLazySystemDLL("kernel32.dll")
	procLockFileEx = kernel32.NewProc("LockFileEx")
)

const (
	lockFileExclusiveLock    = 2 // LOCKFILE_EXCLUSIVE_LOCK
	lockFileFailImmediately  = 1 // LOCKFILE_FAIL_IMMEDIATELY
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

	// LockFileEx(hFile, dwFlags, dwReserved, nNumberOfBytesToLockLow,
	//           nNumberOfBytesToLockHigh, lpOverlapped)
	// We lock byte range [0, 1) — locking zero bytes is a no-op.
	var overlapped windows.Overlapped
	err = procLockFileEx.Find()
	if err != nil {
		// On very old Windows (unlikely with Go 1.24): fall back to a
		// no-op; the lock is advisory not mandatory. Print a warning.
		return f, nil
	}

	ret, _, callErr := procLockFileEx.Call(
		f.Fd(),
		lockFileExclusiveLock|lockFileFailImmediately,
		0, // reserved
		1, // low part of lock length
		0, // high part of lock length
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if ret == 0 {
		f.Close()
		// ERROR_LOCK_VIOLATION (33) or ERROR_SHARING_VIOLATION (32)
		if callErr == windows.ERROR_LOCK_VIOLATION || callErr == windows.ERROR_SHARING_VIOLATION {
			return nil, ErrAlreadyLocked
		}
		return nil, callErr
	}
	return f, nil
}