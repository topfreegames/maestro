package operation

import (
	"time"

	"github.com/topfreegames/maestro/internal/adapters/metrics"
	"github.com/topfreegames/maestro/internal/core/monitoring"
)

const OperationFlowStorageLabel = "operation-flow-storage"

func runRedisOperationReportingLatencyMetric(redisOperation string, executionFunction func()) {
	start := time.Now()
	executionFunction()
	monitoring.ReportLatencyMetricInMillis(
		metrics.PostgresLatencyMetric, start, OperationFlowStorageLabel, redisOperation,
	)
}
