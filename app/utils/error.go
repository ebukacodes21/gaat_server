package utils

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	// "github.com/go-playground/validator/v10"
	"fmt"
)

func ErrorRes(err error) gin.H {
	return gin.H{"message": err.Error()}
}

type ErrorCategory int

const (
	CategoryUnknown ErrorCategory = iota
	CategoryNotFound
	CategoryAlreadyExists
	CategoryInvalidArgument
	CategoryUnauthenticated
	CategoryPermissionDenied
	CategoryInternal
	CategoryTooManyAttempt
)

// the domain/service layer should only return semantic business
// failures and must remain unaware of delivery protocols such as
// HTTP, gRPC, GraphQL, etc.
// transport layers are responsible for translating this error into
// protocol-specific responses (e.g. HTTP status codes or gRPC codes)
// while preserving the original business meaning.
// DomainError represents a transport-agnostic business error.
type DomainError struct {
	Category ErrorCategory
	Message  string
	Err      error
}

func (e *DomainError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewInvalidArgumentError(msg string) error {
	return &DomainError{
		Category: CategoryInvalidArgument,
		Message:  msg,
	}
}

func NewInternalError(msg string, err error) error {
	return &DomainError{
		Category: CategoryInternal,
		Message:  msg,
		Err:      err,
	}
}

func NewNotFoundError(msg string, err error) error {
	return &DomainError{
		Category: CategoryNotFound,
		Message:  msg,
		Err:      err,
	}
}

func NewAlreadyExistsError(msg string, err error) error {
	return &DomainError{
		Category: CategoryAlreadyExists,
		Message:  msg,
		Err:      err,
	}
}

func NewUnathenticatedError(msg string) error {
	return &DomainError{
		Category: CategoryUnauthenticated,
		Message:  msg,
	}
}

func NewPermissionDeniedError(msg string) error {
	return &DomainError{
		Category: CategoryPermissionDenied,
		Message:  msg,
	}
}

func NewTooManyAttemptError(msg string, err error) error {
	return &DomainError{
		Category: CategoryTooManyAttempt,
		Message:  msg,
		Err:      err,
	}
}

func TranslateDomainError(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}

	var dErr *DomainError
	if !errors.As(err, &dErr) {
		return http.StatusInternalServerError, "an internal error occurred. try again"
	}

	switch dErr.Category {
	case CategoryInvalidArgument:
		return http.StatusBadRequest, dErr.Message
	case CategoryNotFound:
		return http.StatusNotFound, dErr.Message
	case CategoryAlreadyExists:
		return http.StatusConflict, dErr.Message
	case CategoryUnauthenticated:
		return http.StatusUnauthorized, dErr.Message
	case CategoryPermissionDenied:
		return http.StatusForbidden, dErr.Message
	case CategoryTooManyAttempt:
		return http.StatusTooManyRequests, dErr.Message
	case CategoryInternal:
		return http.StatusInternalServerError, dErr.Message
	default:
		return http.StatusInternalServerError, "An internal server error occurred."
	}
}
