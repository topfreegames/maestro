package http

// Client is a contract that http reporters clients must implement
type Client interface {
	Send(opts map[string]interface{}) error
}
