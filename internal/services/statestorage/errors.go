package statestorage

import "fmt"

type StateStorageError interface {
	error

	Kind() ErrorKind
}

type ErrorKind int

const (
	ErrorInternal ErrorKind = iota
	ErrorRoomNotFound
	ErrorRoomAlreadyExists
)

type stateStorageError struct {
	kind ErrorKind
	msg  string
	err  error
}

var _ StateStorageError = (*stateStorageError)(nil)

func (e stateStorageError) Unwrap() error {
	return e.err
}

func (e stateStorageError) Error() string {
	return fmt.Sprintf("error: %s, reason: %s", e.msg, e.err)
}

func (e stateStorageError) Kind() ErrorKind {
	return e.kind
}

func WrapError(msg string, err error) StateStorageError {
	return &stateStorageError{
		kind: ErrorInternal,
		msg:  msg,
		err:  err,
	}
}

func NewRoomNotFoundError(scheduler, roomID string) StateStorageError {
	return &stateStorageError{
		kind: ErrorRoomNotFound,
		msg:  fmt.Sprintf("room %s not found for scheduler %s", roomID, scheduler),
	}
}

func NewRoomAlreadyExistsError(scheduler, roomID string) StateStorageError {
	return &stateStorageError{
		kind: ErrorRoomAlreadyExists,
		msg:  fmt.Sprintf("room %s already exists in scheduler %s", roomID, scheduler),
	}
}
