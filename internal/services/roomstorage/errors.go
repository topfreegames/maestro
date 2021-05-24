package roomstorage

import "fmt"

type errorKind int

const (
	errorKindInternal errorKind = iota
	errorKindRoomNotFound
	errorKindRoomAlreadyExists
)

var (
	InternalError = &roomStorageError{kind: errorKindInternal}
	RoomNotFoundError = &roomStorageError{kind: errorKindRoomNotFound}
	RoomAlreadyExistsError = &roomStorageError{kind: errorKindRoomAlreadyExists}
)

type roomStorageError struct {
	kind errorKind
	msg  string
	err  error
}

func (e *roomStorageError) Unwrap() error {
	return e.err
}

func (e *roomStorageError) Error() string {
	return fmt.Sprintf("error: %s, reason: %s", e.msg, e.err)
}

func (e *roomStorageError) Is(other error) bool {
	if err, ok := other.(*roomStorageError); ok {
		return e.kind == err.kind
	}
	return false
}

func WrapError(msg string, err error) *roomStorageError {
	return &roomStorageError{
		kind: errorKindInternal,
		msg:  msg,
		err:  err,
	}
}

func NewRoomNotFoundError(scheduler, roomID string) *roomStorageError {
	return &roomStorageError{
		kind: errorKindRoomNotFound,
		msg:  fmt.Sprintf("room %s not found for scheduler %s", roomID, scheduler),
	}
}

func NewRoomAlreadyExistsError(scheduler, roomID string) *roomStorageError {
	return &roomStorageError{
		kind: errorKindRoomAlreadyExists,
		msg:  fmt.Sprintf("room %s already exists in scheduler %s", roomID, scheduler),
	}
}
