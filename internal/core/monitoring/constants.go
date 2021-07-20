package monitoring

const (
	Namespace       = "maestro"
	SubsystemApi    = "api"
	SubsystemWorker = "worker"
)

const (
	LabelPlatform            = "platform"
	LabelCompanyID           = "company_id"
	LabelBundleID            = "bundle_id"
	LabelProductID           = "product_id"
	LabelEventType           = "event_type"
	LabelField               = "field"
	LabelAnomaly             = "anomaly"
	LabelTopic               = "topic"
	LabelPartition           = "partition"
	LabelSource              = "source"
	LabelAck                 = "ack"
	LabelSuccess             = "success"
	LabelReason              = "reason"
	LabelErrorText           = "error_text"
	LabelUnprocessedReceipts = "unprocessed_receipts"
	LabelLastPhase           = "phase"
	LabelFallback            = "fallback"
	LabelServer              = "server"
	LabelHTTPRoute           = "http_route"
	LabelStatusCode          = "status_code"
	LabelScheduler           = "scheduler"
	LabelOperation           = "operation"
)

const (
	IntValueUnknown    = -1
	StringValueUnknown = "unknown"
)

// DefBucketsMs is similar to prometheus.DefBuckets, but tailored for milliseconds instead of seconds.
var DefBucketsMs = []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

var DefaultLatencyObjectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
