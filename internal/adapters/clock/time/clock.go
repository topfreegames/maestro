package time

import (
	"time"

	"github.com/topfreegames/maestro/internal/core/ports"
)

type clock struct{}

func (c clock) Now() time.Time {
	return time.Now()
}

func NewClock() ports.Clock {
	return clock{}
}
