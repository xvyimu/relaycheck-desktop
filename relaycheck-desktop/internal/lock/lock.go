package lock

import "errors"

var ErrAlreadyLocked = errors.New("another instance is already running")