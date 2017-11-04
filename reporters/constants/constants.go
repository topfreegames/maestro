package constants

// Constants for event metrics possible of being reported
const (
	EventGruNew    = "gru.new"
	EventGruPing   = "gru.ping"
	EventGruDelete = "gru.delete"
	EventGruStatus = "gru.status"

	EventRPCStatus = "rpc.status"
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

//Tag{...} represents datadog tags
const (
	TagGame      = "maestro-game"
	TagScheduler = "maestro-scheduler"
	TagRegion    = "maestro-region"
	TagReason    = "maestro-reason"
	TagStatus    = "maestro-status"
)
