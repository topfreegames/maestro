package operation

import (
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/topfreegames/maestro/internal/adapters/metrics"
	"github.com/topfreegames/maestro/internal/core/monitoring"
)

const OperationFlowStorageLabel = "operation-flow-storage"
const OperationLeaseStorageLabel = "operation-lease-storage"

func runWithMetrics(storage string, executionFunction func() error) {
	start := time.Now()
	err := executionFunction()
	if err != nil && err != redis.Nil {
		reportOperationFlowStorageFailsCounterMetric(storage)
	}
	monitoring.ReportLatencyMetricInMillis(
		metrics.RedisLatencyMetric, start, storage,
	)
}

func reportOperationFlowStorageFailsCounterMetric(storage string) {
	metrics.RedisFailsCounterMetric.WithLabelValues(storage).Inc()
}
