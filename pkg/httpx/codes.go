package httpx

import (
	"fmt"
)

// ErrorCode is a machine-readable error code for API clients.
type ErrorCode string

const (
	// 4xx client errors
	ErrCodeBadRequest        ErrorCode = "BAD_REQUEST"
	ErrCodeInvalidInput      ErrorCode = "INVALID_INPUT"
	ErrCodeUnauthorized      ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden         ErrorCode = "FORBIDDEN"
	ErrCodeNotFound          ErrorCode = "NOT_FOUND"
	ErrCodeConflict          ErrorCode = "CONFLICT"
	ErrCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeValidationFailed  ErrorCode = "VALIDATION_FAILED"
	ErrCodeSessionExpired    ErrorCode = "SESSION_EXPIRED"
	ErrCodeSessionInvalid    ErrorCode = "SESSION_INVALID"
	ErrCodeTokenRequired     ErrorCode = "TOKEN_REQUIRED"
	ErrCodeTokenInvalid      ErrorCode = "TOKEN_INVALID"
	ErrCodeRoleForbidden     ErrorCode = "ROLE_FORBIDDEN"
	ErrCodeAdminAccessDenied ErrorCode = "ADMIN_ACCESS_DENIED"

	// 5xx server errors
	ErrCodeInternal           ErrorCode = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeGatewayTimeout     ErrorCode = "GATEWAY_TIMEOUT"
)

// AppError wraps a domain error with HTTP semantics.
// Message is safe to display to clients; Err is for internal logging only.
type AppError struct {
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	Err     error       `json:"-"` // internal only — never serialized
}

// Error implements the error interface. Returns the safe Message (not Err).
func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// Unwrap returns the wrapped internal error (for errors.Is/As).
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// WithDetails attaches structured details to the error response.
func (e *AppError) WithDetails(details interface{}) *AppError {
	e.Details = details
	return e
}

// Constructor helpers — all return AppError with safe messages.

// NewBadRequestf creates a 400 with a formatted message.
func NewBadRequestf(format string, args ...any) *AppError {
	return &AppError{Code: ErrCodeBadRequest, Message: fmt.Sprintf(format, args...)}
}

// NewInvalidInput creates a 400 with INVALID_INPUT code.
func NewInvalidInput(message string) *AppError {
	return &AppError{Code: ErrCodeInvalidInput, Message: message}
}

// NewUnauthorized creates a 401.
func NewUnauthorized(message string) *AppError {
	if message == "" {
		message = "Authentication required"
	}
	return &AppError{Code: ErrCodeUnauthorized, Message: message}
}

// NewForbidden creates a 403.
func NewForbidden(message string) *AppError {
	if message == "" {
		message = "You do not have permission to access this resource"
	}
	return &AppError{Code: ErrCodeForbidden, Message: message}
}

// NewNotFound creates a 404 with a formatted resource name.
func NewNotFound(resource string) *AppError {
	if resource == "" {
		resource = "Resource"
	}
	return &AppError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("%s not found", resource),
	}
}

// NewConflict creates a 409.
func NewConflict(message string) *AppError {
	return &AppError{Code: ErrCodeConflict, Message: message}
}

// NewTooManyRequests creates a 429.
func NewTooManyRequests(retryAfter int) *AppError {
	return &AppError{
		Code:    ErrCodeRateLimitExceeded,
		Message: "Rate limit exceeded. Please slow down.",
		Details: map[string]int{"retry_after": retryAfter},
	}
}

// NewInternalError creates a 500 with a GENERIC message.
// The original error is preserved for server-side logging via Unwrap().
// NEVER expose the underlying err.Error() to clients.
func NewInternalError(err error) *AppError {
	return &AppError{
		Code:    ErrCodeInternal,
		Message: "An internal error occurred. Please try again later.",
		Err:     err,
	}
}

// NewServiceUnavailable creates a 503.
func NewServiceUnavailable(message string) *AppError {
	if message == "" {
		message = "Service temporarily unavailable"
	}
	return &AppError{Code: ErrCodeServiceUnavailable, Message: message}
}

// NewGatewayTimeout creates a 504.
func NewGatewayTimeout() *AppError {
	return &AppError{Code: ErrCodeGatewayTimeout, Message: "Gateway timeout"}
}

// ─── Convenience aliases (backward compat) ─────────────────────────────────
// Use New* versions; these keep older call sites working.

func BadRequestf(format string, args ...any) *AppError       { return NewBadRequestf(format, args...) }
func InvalidInput(message string) *AppError                  { return NewInvalidInput(message) }
func Unauthorized(message string) *AppError                  { return NewUnauthorized(message) }
func Forbidden(message string) *AppError                     { return NewForbidden(message) }
func NotFound(resource string) *AppError                     { return NewNotFound(resource) }
func Conflict(message string) *AppError                      { return NewConflict(message) }
func TooManyRequests(retryAfter int) *AppError               { return NewTooManyRequests(retryAfter) }
func InternalError(err error) *AppError                      { return NewInternalError(err) }
func ServiceUnavailable(message string) *AppError            { return NewServiceUnavailable(message) }
func GatewayTimeout() *AppError                              { return NewGatewayTimeout() }