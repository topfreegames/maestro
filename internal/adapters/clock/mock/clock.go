package mock

import (
	"time"

	"github.com/topfreegames/maestro/internal/core/ports"
)

type fakeClock struct {
	now time.Time
}

func NewFakeClock(now time.Time) ports.Clock {
	return fakeClock{now: now}
}

func (c fakeClock) Now() time.Time {
	return c.now
}
