package accounts

import (
	"errors"
	"fmt"
)

var (
	ErrSQLitePathRejected        = errors.New("sqlite import path rejected")
	ErrImportInvalidFormat       = errors.New("import source has invalid format")
	ErrImportUpstreamAuth        = errors.New("import upstream authentication failed")
	ErrImportUpstreamUnavailable = errors.New("import upstream unavailable")
	ErrImportStorage             = errors.New("import storage failed")
)

func wrapImportError(kind error, cause error) error {
	if cause == nil {
		return kind
	}
	return fmt.Errorf("%w: %v", kind, cause)
}
