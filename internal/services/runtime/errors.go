package runtime

import (
	"fmt"
)

type errorKind int

const (
	schedulerAlreadyExists errorKind = iota
	schedulerNotFound
	gameRoomConversion
	gameRoomNotFound
	invalidGameRoomSpec
	unknown
)

var (
	ErrSchedulerAlreadyExists = &runtimeError{kind: schedulerAlreadyExists}
	ErrSchedulerNotFound      = &runtimeError{kind: schedulerNotFound}
	ErrGameRoomConversion     = &runtimeError{kind: gameRoomConversion}
	ErrGameRoomNotFound       = &runtimeError{kind: gameRoomNotFound}
	ErrInvalidGameRoomSpec    = &runtimeError{kind: invalidGameRoomSpec}
	ErrUnknown                = &runtimeError{kind: unknown}
)

type runtimeError struct {
	kind    errorKind
	message string
	err     error
}

func (re *runtimeError) Unwrap() error {
	return re.err
}

func (re *runtimeError) Error() string {
	if re.err == nil {
		return re.message
	}

	return fmt.Sprintf("%s: %s", re.message, re.err)
}

func (re *runtimeError) Is(other error) bool {
	if err, ok := other.(*runtimeError); ok {
		return re.kind == err.kind
	}

	return false
}

func NewErrSchedulerAlreadyExists(schedulerID string) *runtimeError {
	return &runtimeError{
		kind:    schedulerAlreadyExists,
		message: fmt.Sprintf("scheduler '%s' already exists", schedulerID),
	}
}

func NewErrSchedulerNotFound(schedulerID string) *runtimeError {
	return &runtimeError{
		kind:    schedulerNotFound,
		message: fmt.Sprintf("scheduler '%s' not found", schedulerID),
	}
}

func NewErrGameRoomConversion(err error) *runtimeError {
	return &runtimeError{
		kind:    gameRoomConversion,
		err:     err,
		message: "room could not be converted to runtime",
	}
}

func NewErrGameRoomNotFound(roomID string) *runtimeError {
	return &runtimeError{
		kind:    gameRoomNotFound,
		message: fmt.Sprintf("room '%s' not found", roomID),
	}
}

func NewErrInvalidGameRoomSpec(roomID string, err error) *runtimeError {
	return &runtimeError{
		kind:    invalidGameRoomSpec,
		err:     err,
		message: fmt.Sprintf("room '%s' has an invalid spec", roomID),
	}
}

func NewErrUnknown(err error) *runtimeError {
	return &runtimeError{
		kind:    unknown,
		err:     err,
		message: "unknown error from runtime",
	}
}
