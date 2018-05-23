package constants

// Constants for event metrics possible of being reported
const (
	EventGruNew    = "gru.new"
	EventGruPing   = "gru.ping"
	EventGruDelete = "gru.delete"
	EventGruStatus = "gru.status"

	EventRPCStatus   = "rpc.status"
	EventRPCDuration = "rpc.duration"

	EventHTTPResponseTime = "http.response.time"

	EventPodStatus     = "pod.status"
	EventPodLastStatus = "pod.last_status"

	EventResponseTime = "response.time"
)

//Reason{...} represents reasons for a pod/gru being deleted
const (
	ReasonScaleDown         = "scale_down"
	ReasonPingTimeout       = "ping_timeout"
	ReasonOccupiedTimeout   = "occupied_timeout"
	ReasonUpdate            = "update"
	ReasonUpdateError       = "update_error"
	ReasonNamespaceDeletion = "namespace_deletion"
	ReasonInvalidPod        = "invalid_pod"
)

//Tag{...} represents datadog tags
const (
	TagGame         = "maestro-game"
	TagScheduler    = "maestro-scheduler"
	TagRegion       = "maestro-region"
	TagReason       = "maestro-reason"
	TagStatus       = "maestro-status"
	TagHostname     = "maestro-hostname"
	TagRoute        = "maestro-route"
	TagResponseTime = "maestro-response-time"
	TagHTTPStatus   = "maestro-http-status"
	TagSegment      = "maestro-segment"
	TagTable        = "maestro-table"
	TagType         = "maestro-type"
	TagError        = "maestro-error"
)

// Value{...} are values that reporters use
const (
	ValueGauge     = "gauge"
	ValueHistogram = "histogram"
	ValueName      = "name"
)
