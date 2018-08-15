package eventforwarder

//Route{...} are the event forwarders routes names
const (
	RoutePlayerEvent = "forwardPlayerEvent"
	RouteRoomInfo    = "forwardRoomInfo"
	RouteRoomEvent   = "forwardRoomEvent"
)

// EventTypes
const (
	PingTimeoutEvent     = "pingTimeout"
	OccupiedTimeoutEvent = "occupiedTimeout"
)
