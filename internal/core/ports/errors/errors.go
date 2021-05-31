package errors

import "fmt"

type errorKind int

const (
	errUnexpected errorKind = iota
	errAlreadyExists
	errNotFound
	errEncoding
	errInvalidArgument
)

var (
	ErrUnexpected      = &portsError{kind: errUnexpected}
	ErrAlreadyExists   = &portsError{kind: errAlreadyExists}
	ErrNotFound        = &portsError{kind: errNotFound}
	ErrEncoding        = &portsError{kind: errEncoding}
	ErrInvalidArgument = &portsError{kind: errInvalidArgument}
)

type portsError struct {
	kind    errorKind
	message string
	err     error
}

func (e *portsError) Unwrap() error {
	return e.err
}

func (e *portsError) Error() string {
	if e.err == nil {
		return e.message
	}

	return fmt.Sprintf("%s: %s", e.message, e.err)
}

func (e *portsError) Is(other error) bool {
	if err, ok := other.(*portsError); ok {
		return e.kind == err.kind
	}

	return false
}

func (e *portsError) WithError(err error) *portsError {
	e.err = err
	return e
}

func NewErrUnexpected(format string, args ...interface{}) *portsError {
	return &portsError{
		kind:    errUnexpected,
		message: fmt.Sprintf(format, args...),
	}
}

func NewErrAlreadyExists(format string, args ...interface{}) *portsError {
	return &portsError{
		kind:    errAlreadyExists,
		message: fmt.Sprintf(format, args...),
	}
}

func NewErrNotFound(format string, args ...interface{}) *portsError {
	return &portsError{
		kind:    errNotFound,
		message: fmt.Sprintf(format, args...),
	}
}

func NewErrEncoding(format string, args ...interface{}) *portsError {
	return &portsError{
		kind:    errEncoding,
		message: fmt.Sprintf(format, args...),
	}
}

func NewErrInvalidArgument(format string, args ...interface{}) *portsError {
	return &portsError{
		kind:    errInvalidArgument,
		message: fmt.Sprintf(format, args...),
	}
}
