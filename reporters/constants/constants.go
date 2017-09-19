package constants

// Constants for event metrics possible of being reported
const (
	EventGruNew     = "gru.new"
	EventGruDelete  = "gru.delete"
	EventRoomStatus = "gru.status"
)

//Reason{...} represents reasons for a pod/gru being deleted
const (
	ReasonScaleDown         = "scale_down"
	ReasonPingTimeout       = "ping_timeout"
	ReasonOccupiedTimeout   = "occupied_timeout"
	ReasonUpdate            = "update"
	ReasonUpdateError       = "update_error"
	ReasonNamespaceDeletion = "namespace_deletion"
)
