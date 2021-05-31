package port_allocator

import "errors"

var (
	ErrPortRangeInvalidFormat     = errors.New("invalid port range format it must follow \"start-end\"")
	ErrPortRangeInvalidValue      = errors.New("invalid port range value")
	ErrPortRangeEndLowerThanStart = errors.New("port range end must be higher than start")
	ErrNotEnoughPorts             = errors.New("port range has not enough ports")
)
