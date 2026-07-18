package core

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// WriteSessionTokenFile persists a process token only when the platform can
// both apply and verify a restrictive file security policy.
func WriteSessionTokenFile(path string, token string) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("session token must not be empty")
	}
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return err
	}
	if err := secureSessionTokenFile(path); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("secure session token file: %w", err)
	}
	if err := verifySessionTokenFileSecurity(path); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("verify session token file security: %w", err)
	}
	return nil
}
