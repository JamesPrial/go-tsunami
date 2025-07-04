package common

import "fmt"

// ProtocolError represents errors in the Tsunami protocol handling
type ProtocolError struct {
	op      string // operation that failed
	message string // error details
	code    ErrorCode
}

// ErrorCode represents different categories of protocol errors
type ErrorCode int

const (
	ErrUnknown ErrorCode = iota
	ErrInvalidFormat
	ErrUnknownInstruction
	ErrValidationFailed
	ErrParseError
)

// Error implements the error interface
func (e *ProtocolError) Error() string {
	return fmt.Sprintf("protocol %s: %s", e.op, e.message)
}

// Unwrap implements error unwrapping for Go 1.13+
func (e *ProtocolError) Unwrap() error {
	return nil // Protocol errors are typically leaf errors
}

// Code returns the error category
func (e *ProtocolError) Code() ErrorCode {
	return e.code
}

// Operation returns the operation that failed
func (e *ProtocolError) Operation() string {
	return e.op
}

// Message returns the error message
func (e *ProtocolError) Message() string {
	return e.message
}

// String returns a human-readable description of the error code
func (c ErrorCode) String() string {
	switch c {
	case ErrInvalidFormat:
		return "invalid_format"
	case ErrUnknownInstruction:
		return "unknown_instruction"
	case ErrValidationFailed:
		return "validation_failed"
	case ErrParseError:
		return "parse_error"
	default:
		return "unknown"
	}
}

// IsParseError returns true if the error is a parsing error
func IsParseError(err error) bool {
	if protocolErr, ok := err.(*ProtocolError); ok {
		return protocolErr.code == ErrParseError
	}
	return false
}

// IsValidationError returns true if the error is a validation error
func IsValidationError(err error) bool {
	if protocolErr, ok := err.(*ProtocolError); ok {
		return protocolErr.code == ErrValidationFailed
	}
	return false
}

// IsProtocolError returns true if the error is any protocol error
func IsProtocolError(err error) bool {
	_, ok := err.(*ProtocolError)
	return ok
}

// Internal helper functions (unexported - implementation details)

func newParseError(op, message string) *ProtocolError {
	return &ProtocolError{
		op:      op,
		message: message,
		code:    ErrParseError,
	}
}

func newValidationError(op, message string) *ProtocolError {
	return &ProtocolError{
		op:      op,
		message: message,
		code:    ErrValidationFailed,
	}
}

func newProtocolError(op, message string) *ProtocolError {
	return &ProtocolError{
		op:      op,
		message: message,
		code:    ErrInvalidFormat,
	}
}
