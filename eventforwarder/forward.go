package eventforwarder

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	redisinterfaces "github.com/topfreegames/extensions/v9/redis/interfaces"
	pginterfaces "github.com/topfreegames/extensions/v9/pg/interfaces"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/reporters"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
	"k8s.io/client-go/kubernetes"
)

// Response is the struct that defines a forwarder response
type Response struct {
	Code    int
	Message string
}

// Forwarder is the struct that defines a forwarder with its EventForwarder as configured
// in maestro and the metadata configured for each scheduler
type Forwarder struct {
	Func           EventForwarder
	ReturnResponse bool
	Metadata       map[string]interface{}
}

func getEnabledForwarders(
	schedulerForwarders map[string]map[string]*models.Forwarder,
	configuredForwarders []*Info,
) []*Forwarder {
	enabledForwarders := make([]*Forwarder, 0)
	for _, configuredFwdInfo := range configuredForwarders {
		if schedulerFwds, ok := schedulerForwarders[configuredFwdInfo.Plugin]; ok {
			if fwd, ok := schedulerFwds[configuredFwdInfo.Name]; ok {
				if fwd.Enabled {
					enabledForwarders = append(
						enabledForwarders,
						&Forwarder{configuredFwdInfo.Forwarder, fwd.ForwardResponse, fwd.Metadata},
					)
				}
			}
		}
	}
	return enabledForwarders
}

// ForwardRoomEvent forwards room event to app eventforwarders
func ForwardRoomEvent(
	ctx context.Context,
	forwarders []*Info,
	redis redisinterfaces.RedisClient,
	db pginterfaces.DB,
	kubernetesClient kubernetes.Interface,
	mr *models.MixedMetricsReporter,
	room *models.Room,
	status string,
	eventType string,
	metadata map[string]interface{},
	schedulerCache *models.SchedulerCache,
	logger logrus.FieldLogger,
	addrGetter models.AddrGetter,
) (res *Response, err error) {
	var eventWasForwarded bool
	var warning error
	startTime := time.Now()
	defer func() {
		reportRPCStatus(
			eventWasForwarded,
			room.SchedulerName, RouteRoomEvent,
			db,
			schedulerCache,
			logger,
			err,
			time.Now().Sub(startTime),
			warning,
		)
	}()

	l := logger.WithFields(logrus.Fields{
		"op":        "forwardRoomEvent",
		"scheduler": room.SchedulerName,
		"roomId":    room.ID,
	})

	if len(metadata) == 0 {
		metadata = map[string]interface{}{}
	}

	if len(forwarders) > 0 {
		l.Debug("forwarding information to app forwarders")

		cachedScheduler, err := schedulerCache.LoadScheduler(db, room.SchedulerName, true)
		if err != nil {
			l.WithError(err).Error("error reading scheduler from cache")
			return nil, err
		}

		l.WithFields(logrus.Fields{
			"schedulerForwarders": len(cachedScheduler.ConfigYAML.Forwarders),
		}).Debug("checking enabled forwarders")

		if len(cachedScheduler.ConfigYAML.Forwarders) > 0 {
			enabledForwarders := getEnabledForwarders(cachedScheduler.ConfigYAML.Forwarders, forwarders)

			l.WithFields(logrus.Fields{
				"schedulerForwarders": len(cachedScheduler.ConfigYAML.Forwarders),
				"enabledForwarders":   len(enabledForwarders),
			}).Debug("got enabled forwarders")

			if len(enabledForwarders) > 0 {
				infos := map[string]interface{}{
					"roomId": room.ID,
					"game":   cachedScheduler.Scheduler.Game,
				}
				if eventType != PingTimeoutEvent && eventType != OccupiedTimeoutEvent {
					infos, err = room.GetRoomInfos(redis, db, kubernetesClient, schedulerCache, cachedScheduler.Scheduler, addrGetter, mr)

					if err != nil {
						l.WithError(err).Error("error getting room info from redis")
						return nil, err
					}
					reportIpv6Status(infos, logger)
				} else { // fill host and port with zero values when pingTimeout or occupiedTimeout event so it won't break the GRPCForwarder
					infos["host"] = ""
					infos["port"] = int32(0)
				}

				if infos["metadata"] == nil {
					infos["metadata"] = make(map[string]interface{}, len(metadata))
				}

				for key, info := range metadata {
					infos["metadata"].(map[string]interface{})[key] = info
				}
				eventWasForwarded = true

				res, warning, err = ForwardEventToForwarders(ctx, enabledForwarders, status, infos, l)
				return res, err
			}
		}
	}

	l.Debug("no forwarders configured and enabled")

	return nil, nil
}

// ForwardPlayerEvent forwards player event to app eventforwarders
func ForwardPlayerEvent(
	ctx context.Context,
	forwarders []*Info,
	db pginterfaces.DB,
	kubernetesClient kubernetes.Interface,
	room *models.Room,
	event string,
	metadata map[string]interface{},
	schedulerCache *models.SchedulerCache,
	logger logrus.FieldLogger,
) (resp *Response, err error) {
	var eventWasForwarded bool
	var warning error
	startTime := time.Now()
	defer func() {
		reportRPCStatus(
			eventWasForwarded,
			room.SchedulerName, RoutePlayerEvent,
			db,
			schedulerCache,
			logger,
			err,
			time.Now().Sub(startTime),
			warning,
		)
	}()

	l := logger.WithFields(logrus.Fields{
		"op":        "forwardPlayerEvent",
		"scheduler": room.SchedulerName,
		"roomId":    room.ID,
	})
	if len(forwarders) > 0 {
		metadata["roomId"] = room.ID
		cachedScheduler, err := schedulerCache.LoadScheduler(db, room.SchedulerName, true)
		if err != nil {
			return nil, err
		}
		metadata["game"] = cachedScheduler.Scheduler.Game
		l.WithFields(logrus.Fields{
			"schedulerForwarders": len(cachedScheduler.ConfigYAML.Forwarders),
		}).Debug("checking enabled forwarders")
		if len(cachedScheduler.ConfigYAML.Forwarders) > 0 {
			enabledForwarders := getEnabledForwarders(cachedScheduler.ConfigYAML.Forwarders, forwarders)
			l.WithFields(logrus.Fields{
				"schedulerForwarders": len(cachedScheduler.ConfigYAML.Forwarders),
				"enabledForwarders":   len(enabledForwarders),
			}).Debug("got enabled forwarders")
			if len(enabledForwarders) > 0 {
				eventWasForwarded = true
				resp, warning, err = ForwardEventToForwarders(ctx, enabledForwarders, event, metadata, l)
				return resp, err
			}
		}
	}
	l.Debug("no forwarders configured and enabled")
	return nil, nil
}

// ForwardEventToForwarders forwards the event, in case wheere `ReturnMessage`
// the forwarder result won't be used. It returns the response which consists in
// the last forwarder response, an error representing a warning (this happen
// when a forwarder that has `ReturnMessage = false` returns an error) and an
// error representing a failure on a `Forward` call.
// TODO: check if a failure on one forwarder should affect all the forwarders
func ForwardEventToForwarders(
	ctx context.Context,
	forwarders []*Forwarder,
	event string,
	infos map[string]interface{},
	logger logrus.FieldLogger,
) (resp *Response, warning error, err error) {
	logger.WithFields(logrus.Fields{
		"forwarders": len(forwarders),
		"infos":      infos,
		"event":      event,
	}).Debug("forwarding events")
	respCode := 0
	respMessage := []string{}
	for _, f := range forwarders {
		code, message, err := f.Func.Forward(ctx, event, infos, f.Metadata)
		if !f.ReturnResponse {
			if err != nil {
				// if there is any error on a forward that does not return the
				// response we set it as a "warning"
				return nil, err, nil
			}

			continue
		}

		if err != nil {
			return nil, nil, err
		}
		// return the highest code received
		// if a forwarder returns 200 and another 500 we should return 500 to indicate the error
		// TODO: consider configuring for each forwarder if its errors can be ignored
		if int(code) > respCode {
			respCode = int(code)
		}
		if message != "" {
			respMessage = append(respMessage, message)
		}
	}

	if respCode == 0 {
		return nil, nil, nil
	}

	resp = &Response{
		Code:    respCode,
		Message: strings.Join(respMessage, ";"),
	}
	return resp, nil, nil
}

// reportRPCStatus sends RPC status to StatsD. In case where
// `eventForwarderErr` or `eventForwarderWarning` are not nil we send it as
// `failure`, otherwise it is sent as `success`.
func reportRPCStatus(
	eventWasForwarded bool,
	schedulerName, forwardRoute string,
	db pginterfaces.DB,
	cache *models.SchedulerCache,
	logger logrus.FieldLogger,
	eventForwarderErr error,
	responseTime time.Duration,
	eventForwarderWarning error,
) {
	if !reporters.HasReporters() {
		return
	}
	log := logger.WithField("operation", "reportRPCStatus")

	if !eventWasForwarded {
		log.Debug("no rpc was made, returning...")
		return
	}

	scheduler, err := cache.LoadScheduler(db, schedulerName, true)
	if err != nil {
		logger.
			WithField("operation", "reportRPCStatus").
			WithError(err).
			Error("failed to report RPC connection to StatsD")
		return
	}

	game := scheduler.ConfigYAML.Game

	status := map[string]interface{}{
		reportersConstants.TagGame:      game,
		reportersConstants.TagScheduler: schedulerName,
		reportersConstants.TagHostname:  Hostname(),
		reportersConstants.TagRoute:     forwardRoute,
		reportersConstants.TagStatus:    "success",
	}

	if eventForwarderErr != nil {
		status[reportersConstants.TagStatus] = "failed"
		status[reportersConstants.TagReason] = eventForwarderErr.Error()
	}

	if eventForwarderWarning != nil {
		status[reportersConstants.TagStatus] = "failed"
		status[reportersConstants.TagReason] = eventForwarderWarning.Error()
	}

	reporterErr := reporters.Report(reportersConstants.EventRPCStatus, status)
	if reporterErr != nil {
		logger.
			WithField("operation", "reportRPCStatus").
			WithError(err).
			Error("failed to report RPC connection to StatsD")
	}

	status[reportersConstants.TagResponseTime] = responseTime.String()
	reporterErr = reporters.Report(reportersConstants.EventRPCDuration, status)
	if reporterErr != nil {
		logger.
			WithField("operation", "reportRPCDuration").
			WithError(err).
			Error("failed to report RPC connection to StatsD")
	}
}

// reportIpv6Status sends to StatsD success true if suceessfuly read
// ipv6 label and false otherwise
func reportIpv6Status(
	infos map[string]interface{},
	logger logrus.FieldLogger,
) {
	if !reporters.HasReporters() {
		return
	}

	status := map[string]interface{}{
		reportersConstants.TagNodeHost: infos["host"].(string),
		reportersConstants.TagStatus:   "success",
	}

	if infos["metadata"] == nil ||
		infos["metadata"].(map[string]interface{})["ipv6Label"] == nil ||
		infos["metadata"].(map[string]interface{})["ipv6Label"].(string) == "" {
		status[reportersConstants.TagStatus] = "failed"
	}

	reporterErr := reporters.Report(reportersConstants.EventNodeIpv6Status, status)
	if reporterErr != nil {
		logger.
			WithField("operation", "reportIpv6Status").
			WithError(reporterErr).
			Error("failed to report Ipv6 read status to StatsD")
	}
}

//Hostname returns the host name.
//If running on Kubernetes, returns the pod name.
func Hostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func toSecondsString(secs time.Duration) string {
	return fmt.Sprint(
		secs.Seconds(),
	)
}
