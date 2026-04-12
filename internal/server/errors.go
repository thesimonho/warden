package server

import (
	"encoding/json"
	"net/http"
)

// ErrorCode is a machine-readable error code returned in JSON error responses.
type ErrorCode string

// Error codes returned in the "code" field of JSON error responses.
// Consumers can match on these instead of parsing human-readable messages.
const (
	// Validation errors (400).
	ErrCodeInvalidBody          ErrorCode = "INVALID_BODY"
	ErrCodeInvalidContainerID   ErrorCode = "INVALID_CONTAINER_ID"
	ErrCodeInvalidContainerName ErrorCode = "INVALID_CONTAINER_NAME"
	ErrCodeInvalidWorktreeID    ErrorCode = "INVALID_WORKTREE_ID"
	ErrCodeInvalidWorktreeName  ErrorCode = "INVALID_WORKTREE_NAME"
	ErrCodeRequiredField        ErrorCode = "REQUIRED_FIELD"
	ErrCodeInvalidPath          ErrorCode = "INVALID_PATH"
	ErrCodeInvalidNetworkConfig ErrorCode = "INVALID_NETWORK_CONFIG"
	ErrCodeNotADirectory        ErrorCode = "NOT_A_DIRECTORY"

	// Resource errors.
	ErrCodeNotFound        ErrorCode = "NOT_FOUND"        // 404
	ErrCodeNameTaken       ErrorCode = "NAME_TAKEN"       // 409
	ErrCodeContainerExists ErrorCode = "CONTAINER_EXISTS" // 409

	// Infrastructure errors.
	ErrCodeNotConfigured  ErrorCode = "NOT_CONFIGURED"  // 503
	ErrCodeStaleMounts    ErrorCode = "STALE_MOUNTS"    // 409
	ErrCodeBudgetExceeded ErrorCode = "BUDGET_EXCEEDED" // 403

	// Proxy errors.
	ErrCodeProxyError ErrorCode = "PROXY_ERROR" // 502

	// Catch-all for unclassified server errors.
	ErrCodeInternal ErrorCode = "INTERNAL" // 500
)

// apiError is the JSON structure for all error responses.
type apiError struct {
	Error string    `json:"error"`
	Code  ErrorCode `json:"code"`
}

// writeError writes a JSON error response with a machine-readable code.
func writeError(w http.ResponseWriter, code ErrorCode, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiError{Error: message, Code: code}) //nolint:errcheck
}
