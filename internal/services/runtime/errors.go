package runtime

import "errors"

var (
	ErrSchedulerAlreadyExists  = errors.New("scheduler already exists")
	ErrSchedulerNotFound       = errors.New("scheduler not found")
	ErrGameRoomConversionError = errors.New("failed to convert game room to runtime")
	ErrGameRoomNotFound        = errors.New("game room not found")
	ErrInvalidGameRoomSpec     = errors.New("game room specification is invalid")
	ErrUnknown                 = errors.New("unknown error from runtime")
)
