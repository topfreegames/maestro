package events

import (
	"time"

	"github.com/topfreegames/maestro/internal/adapters/metrics"
	"github.com/topfreegames/maestro/internal/core/monitoring"
	pb "github.com/topfreegames/protos/maestro/grpc/generated"
	"google.golang.org/grpc/status"
)

const forwarderServiceMetricLabel = "GRPCForwarder"

func runReportingLatencyMetrics(method string, executionFunction func() (*pb.Response, error)) (*pb.Response, error) {
	start := time.Now()
	response, responseErr := executionFunction()
	statusCode := status.Code(responseErr)

	monitoring.ReportLatencyMetricInMillis(
		metrics.GrpcLatencyMetric, start, forwarderServiceMetricLabel, method, statusCode.String(),
	)
	return response, responseErr
}
