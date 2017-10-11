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

func getEnabledForwarders(
	schedulerForwarders map[string]map[string]*models.Forwarder,
	configuredForwarders []*Info,
) []EventForwarder {
	enabledForwarders := make([]EventForwarder, 0)
	for _, configuredFwdInfo := range configuredForwarders {
		if schedulerFwds, ok := schedulerForwarders[configuredFwdInfo.Plugin]; ok {
			if fwd, ok := schedulerFwds[configuredFwdInfo.Name]; ok {
				if fwd.Enabled {
					enabledForwarders = append(enabledForwarders, configuredFwdInfo.Forwarder)
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
		scheduler, err := schedulerCache.LoadScheduler(db, room.SchedulerName, true)
		if err != nil {
			return nil, err
		}
		var config *models.ConfigYAML
		if schedulerCache != nil {
			config, err = schedulerCache.LoadConfigYaml(db, room.SchedulerName, true)
		} else {
			config, err = models.NewConfigYAML(scheduler.YAML)
		}
		if err != nil {
			return nil, err
		}
		infos, err := room.GetRoomInfos(db, kubernetesClient, schedulerCache, scheduler)
		if err != nil {
			return nil, err
		}
		infos["metadata"] = metadata

		l.WithFields(logrus.Fields{
			"schedulerForwarders": config.Forwarders,
		}).Debug("checking enabled forwarders")
		if len(config.Forwarders) > 0 {
			enabledForwarders := getEnabledForwarders(config.Forwarders, forwarders)
			l.WithFields(logrus.Fields{
				"schedulerForwarders": config.Forwarders,
				"enabledForwarders":   enabledForwarders,
			}).Debug("got enabled forwarders")
			if len(enabledForwarders) > 0 {
				return ForwardEventToForwarders(enabledForwarders, status, infos)
			}
		}
	}
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
		scheduler, err := schedulerCache.LoadScheduler(db, schedulerName, true)
		if err != nil {
			return nil, err
		}
		var config *models.ConfigYAML
		if schedulerCache != nil {
			config, err = schedulerCache.LoadConfigYaml(db, schedulerName, true)
		} else {
			config, err = models.NewConfigYAML(scheduler.YAML)
		}
		if err != nil {
			return nil, err
		}

		l.WithFields(logrus.Fields{
			"schedulerForwarders": config.Forwarders,
		}).Debug("checking enabled forwarders")
		if len(config.Forwarders) > 0 {
			respCode := 0
			respMessage := []string{}
			for _, configuredFwdInfo := range forwarders {
				if schedulerFwds, ok := config.Forwarders[configuredFwdInfo.Plugin]; ok {
					if fwd, ok := schedulerFwds[configuredFwdInfo.Name]; ok {
						if fwd.Enabled {
							l.WithFields(logrus.Fields{
								"schedulerForwarders": config.Forwarders,
								"forwarder":           fwd,
							}).Debug("enabled forwarder")
							metadata := fwd.Metadata
							if metadata != nil {
								metadata["game"] = scheduler.Game
							} else {
								metadata = map[string]interface{}{
									"game": scheduler.Game,
								}
							}

							resp, err := ForwardEventToForwarders(
								[]EventForwarder{configuredFwdInfo.Forwarder},
								"schedulerEvent",
								metadata,
							)
							if err != nil {
								return nil, err
							}
							if resp.Code > respCode {
								respCode = resp.Code
							}
							if resp.Message != "" {
								respMessage = append(respMessage, resp.Message)
							}
						}
					}
				}
			}

			if respCode != 0 {
				response := &Response{
					Code:    respCode,
					Message: strings.Join(respMessage, ";"),
				}
				return response, nil
			}
			return nil, nil
		}
	}
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
		"op":        "forwardRoomEvent",
		"scheduler": room.SchedulerName,
		"roomId":    room.ID,
	})
	if len(forwarders) > 0 {
		metadata["roomId"] = room.ID
		scheduler, err := schedulerCache.LoadScheduler(db, room.SchedulerName, true)
		if err != nil {
			return nil, err
		}
		metadata["game"] = scheduler.Game
		var config *models.ConfigYAML
		if schedulerCache != nil {
			config, err = schedulerCache.LoadConfigYaml(db, room.SchedulerName, true)
		} else {
			config, err = models.NewConfigYAML(scheduler.YAML)
		}
		if err != nil {
			return nil, err
		}
		l.WithFields(logrus.Fields{
			"schedulerForwarders": config.Forwarders,
		}).Debug("checking enabled forwarders")
		if len(config.Forwarders) > 0 {
			enabledForwarders := getEnabledForwarders(config.Forwarders, forwarders)
			l.WithFields(logrus.Fields{
				"schedulerForwarders": config.Forwarders,
				"enabledForwarders":   enabledForwarders,
			}).Debug("got enabled forwarders")
			if len(enabledForwarders) > 0 {
				return ForwardEventToForwarders(enabledForwarders, event, metadata)
			}
		}
	}
	return nil, nil
}

// ForwardEventToForwarders forwards
func ForwardEventToForwarders(forwarders []EventForwarder, event string, infos map[string]interface{}) (*Response, error) {
	respCode := 0
	respMessage := []string{}
	for _, f := range forwarders {
		code, message, err := f.Forward(event, infos)
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
