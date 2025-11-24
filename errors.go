package pubsub

import (
	"errors"
	"fmt"
)

// Error represents a pubsub library error with categorization.
type Error struct {
	// Code is a machine-readable error code
	Code string

	// Message is a human-readable error message
	Message string

	// Err is the underlying error (if any)
	Err error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Err
}

// Error codes for pubsub operations.
const (
	// ErrCodeNoData indicates no data was found.
	ErrCodeNoData = "NO_DATA"

	// ErrCodeValidation indicates validation failed.
	ErrCodeValidation = "VALIDATION_ERROR"

	// ErrCodeConfiguration indicates invalid configuration.
	ErrCodeConfiguration = "CONFIGURATION_ERROR"

	// ErrCodeDatabase indicates database operation failed.
	ErrCodeDatabase = "DATABASE_ERROR"

	// ErrCodeDelivery indicates message delivery failed.
	ErrCodeDelivery = "DELIVERY_ERROR"
)

// Common errors.
var (
	// ErrNoData is returned when a query returns no results.
	// This is not necessarily an error condition in all cases.
	ErrNoData = &Error{
		Code:    ErrCodeNoData,
		Message: "no data found",
	}

	// ErrInvalidConfiguration is returned when worker configuration is invalid.
	ErrInvalidConfiguration = &Error{
		Code:    ErrCodeConfiguration,
		Message: "invalid worker configuration",
	}
)

// NewError creates a new Error with the given code and message.
func NewError(code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithCause creates a new Error wrapping an underlying error.
func NewErrorWithCause(code, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     cause,
	}
}

// IsNoData checks if an error is ErrNoData.
func IsNoData(err error) bool {
	var pubsubErr *Error
	if errors.As(err, &pubsubErr) {
		return pubsubErr.Code == ErrCodeNoData
	}
	return errors.Is(err, ErrNoData)
}
