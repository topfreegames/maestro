package instance_storage

import "fmt"

type errorKind int

const (
	errorKindInternal errorKind = iota
	errorKindInstanceNotFound
)

var (
	InternalError         = &instanceStorageError{kind: errorKindInternal}
	InstanceNotFoundError = &instanceStorageError{kind: errorKindInstanceNotFound}
)

type instanceStorageError struct {
	kind errorKind
	msg  string
	err  error
}

func (e *instanceStorageError) Unwrap() error {
	return e.err
}

func (e *instanceStorageError) Error() string {
	return fmt.Sprintf("error: %s, reason: %s", e.msg, e.err)
}

func (e *instanceStorageError) Is(other error) bool {
	if err, ok := other.(*instanceStorageError); ok {
		return e.kind == err.kind
	}
	return false
}

func WrapError(msg string, err error) *instanceStorageError {
	return &instanceStorageError{
		kind: errorKindInternal,
		msg:  msg,
		err:  err,
	}
}

func NewInstanceNotFoundError(scheduler, roomID string) *instanceStorageError {
	return &instanceStorageError{
		kind: errorKindInstanceNotFound,
		msg:  fmt.Sprintf("room %s not found for scheduler %s", roomID, scheduler),
	}
}
