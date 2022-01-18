package forwarder

import "github.com/topfreegames/maestro/internal/validations"

type FwdType string

const (
	TypeGrpc FwdType = "gRPC"
)

type Forwarder struct {
	Name    string `validate:"required"`
	Enabled bool
	FwdType FwdType `validate:"required"`
	Address string  `validate:"required"`
	Options *FwdOptions
}

func NewForwarder(name string, enabled bool, fwdType FwdType, address string, options *FwdOptions) (*Forwarder, error) {
	forwarder := &Forwarder{
		Name:    name,
		Enabled: enabled,
		FwdType: fwdType,
		Address: address,
		Options: options,
	}
	validationErr := forwarder.Validate()
	if validationErr != nil {
		return nil, validationErr
	}
	return forwarder, nil
}

func (f *Forwarder) Validate() error {
	return validations.Validate.Struct(f)
}
