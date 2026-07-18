//go:build !windows

package core

import (
	"fmt"
	"os"
)

func secureSessionTokenFile(path string) error {
	return secureInstanceKeyFile(path)
}

func secureInstanceKeyFile(path string) error {
	return os.Chmod(path, 0o600)
}

func verifySessionTokenFileSecurity(path string) error {
	return verifyInstanceKeyFileSecurity(path)
}

func verifyInstanceKeyFileSecurity(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		return fmt.Errorf("permissions = %04o, want 0600", permissions)
	}
	return nil
}
