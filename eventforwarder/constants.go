package eventforwarder

//Route{...} are the event forwarders routes names
const (
	RoutePlayerEvent = "forwardPlayerEvent"
	RouteRoomEvent   = "forwardRoomEvent"
)

// EventTypes
const (
	PingTimeoutEvent     = "pingTimeout"
	OccupiedTimeoutEvent = "occupiedTimeout"
)
