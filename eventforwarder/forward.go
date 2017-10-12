package eventforwarder

import (
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/models"
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
	Func     EventForwarder
	Metadata map[string]interface{}
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
						&Forwarder{configuredFwdInfo.Forwarder, fwd.Metadata},
					)
				}
			}
		}
	}
	return enabledForwarders
}

// ForwardRoomEvent forwards room event to app eventforwarders
func ForwardRoomEvent(
	forwarders []*Info,
	db interfaces.DB,
	kubernetesClient kubernetes.Interface,
	room *models.Room,
	status string,
	metadata map[string]interface{},
	schedulerCache *models.SchedulerCache,
	logger logrus.FieldLogger,
) (*Response, error) {
	l := logger.WithFields(logrus.Fields{
		"op":        "forwardRoomEvent",
		"scheduler": room.SchedulerName,
		"roomId":    room.ID,
	})
	if len(forwarders) > 0 {
		cachedScheduler, err := schedulerCache.LoadScheduler(db, room.SchedulerName, true)
		if err != nil {
			return nil, err
		}
		infos, err := room.GetRoomInfos(db, kubernetesClient, schedulerCache, cachedScheduler.Scheduler)
		if err != nil {
			return nil, err
		}
		infos["metadata"] = metadata

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
				return ForwardEventToForwarders(enabledForwarders, status, infos, l)
			}
		}
	}
	l.Debug("no forwarders configured and enabled")
	return nil, nil
}

// ForwardRoomInfo forwards room info to app eventforwarders
func ForwardRoomInfo(
	forwarders []*Info,
	db interfaces.DB,
	kubernetesClient kubernetes.Interface,
	schedulerName string,
	schedulerCache *models.SchedulerCache,
	logger logrus.FieldLogger,
) (*Response, error) {
	l := logger.WithFields(logrus.Fields{
		"op":        "forwardRoomInfo",
		"scheduler": schedulerName,
	})
	if len(forwarders) > 0 {
		cachedScheduler, err := schedulerCache.LoadScheduler(db, schedulerName, true)
		if err != nil {
			return nil, err
		}

		infos := map[string]interface{}{
			"game": cachedScheduler.Scheduler.Game,
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
				return ForwardEventToForwarders(enabledForwarders, "schedulerEvent", infos, l)
			}
		}
	}
	l.Debug("no forwarders configured and enabled")
	return nil, nil
}

// ForwardPlayerEvent forwards player event to app eventforwarders
func ForwardPlayerEvent(
	forwarders []*Info,
	db interfaces.DB,
	kubernetesClient kubernetes.Interface,
	room *models.Room,
	event string,
	metadata map[string]interface{},
	schedulerCache *models.SchedulerCache,
	logger logrus.FieldLogger,
) (*Response, error) {
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
				return ForwardEventToForwarders(enabledForwarders, event, metadata, l)
			}
		}
	}
	l.Debug("no forwarders configured and enabled")
	return nil, nil
}

// ForwardEventToForwarders forwards
func ForwardEventToForwarders(
	forwarders []*Forwarder,
	event string,
	infos map[string]interface{},
	logger logrus.FieldLogger,
) (*Response, error) {
	logger.WithFields(logrus.Fields{
		"forwarders": len(forwarders),
		"infos":      infos,
		"event":      event,
	}).Debug("forwarding events")
	respCode := 0
	respMessage := []string{}
	for _, f := range forwarders {
		code, message, err := f.Func.Forward(event, infos, f.Metadata)
		if err != nil {
			return nil, err
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
	resp := &Response{
		Code:    respCode,
		Message: strings.Join(respMessage, ";"),
	}
	return resp, nil
}
