package forwarder

import (
	"time"
)

type FwdOptions struct {
	Timeout  time.Duration `validate:"required"`
	Metadata map[string]interface{}
}
