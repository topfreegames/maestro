package auth

type AuthenticationResult int

const (
	AuthenticationOk AuthenticationResult = iota
	AuthenticationInvalid
	AuthenticationMissing
	AuthenticationError
)
