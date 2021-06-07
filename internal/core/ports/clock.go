package ports

import "time"

type Clock interface {
	Now() time.Time
}
