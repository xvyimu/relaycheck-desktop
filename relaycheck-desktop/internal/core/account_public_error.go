package core

import (
	"log"
	"strings"
)

// publicAccountFailure records only non-sensitive context and the Go error
// type. Upstream bodies, filesystem paths and credentials must never be copied
// into API result messages or logs.
func publicAccountFailure(operation string, accountID string, publicMessage string, err error) string {
	return publicOperationFailure("accounts", operation, accountID, publicMessage, err)
}

// publicOperationFailure is the shared boundary for business-result errors.
// It records stable identifiers plus the Go error type, never the raw cause.
func publicOperationFailure(component string, operation string, entityID string, publicMessage string, err error) string {
	log.Printf(
		"[%s] operation failed operation=%s entityId=%s causeType=%T",
		strings.TrimSpace(component),
		strings.TrimSpace(operation),
		strings.TrimSpace(entityID),
		err,
	)
	return publicMessage
}
