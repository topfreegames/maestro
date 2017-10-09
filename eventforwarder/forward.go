package eventforwarder

import (
	"fmt"

	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
)

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
) error {
	if len(forwarders) > 0 {
		scheduler, err := schedulerCache.LoadScheduler(db, room.SchedulerName, true)
		if err != nil {
			return err
		}
		var config *models.ConfigYAML
		if schedulerCache != nil {
			config, err = schedulerCache.LoadConfigYaml(db, room.SchedulerName, true)
		} else {
			config, err = models.NewConfigYAML(scheduler.YAML)
		}
		if err != nil {
			return err
		}
		infos, err := room.GetRoomInfos(db, kubernetesClient, schedulerCache, scheduler)
		if err != nil {
			return err
		}
		infos["metadata"] = metadata
		if len(config.Forwarders) > 0 {
			enabledForwarders := getEnabledForwarders(config.Forwarders, forwarders)
			if len(enabledForwarders) > 0 {
				return ForwardEventToForwarders(enabledForwarders, status, infos)
			}
		}
	}
	return nil
}

// ForwardRoomInfo forwards room info to app eventforwarders
func ForwardRoomInfo(
	forwarders []*Info,
	db interfaces.DB,
	kubernetesClient kubernetes.Interface,
	schedulerName string,
	schedulerCache *models.SchedulerCache,
) error {
	if len(forwarders) > 0 {
		scheduler, err := schedulerCache.LoadScheduler(db, schedulerName, true)
		if err != nil {
			return err
		}
		var config *models.ConfigYAML
		if schedulerCache != nil {
			config, err = schedulerCache.LoadConfigYaml(db, schedulerName, true)
		} else {
			config, err = models.NewConfigYAML(scheduler.YAML)
		}
		if len(config.Forwarders) > 0 {
			for _, configuredFwdInfo := range forwarders {
				if schedulerFwds, ok := config.Forwarders[configuredFwdInfo.Plugin]; ok {
					if fwd, ok := schedulerFwds[configuredFwdInfo.Name]; ok {
						if fwd.Enabled {
							fmt.Println(fwd)
							metadata := fwd.Metadata
							if metadata != nil {
								metadata["game"] = scheduler.Game
							} else {
								metadata = map[string]interface{}{
									"game": scheduler.Game,
								}
							}
							err = ForwardEventToForwarders(
								[]EventForwarder{configuredFwdInfo.Forwarder},
								"schedulerEvent",
								metadata,
							)
							if err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}
	return nil
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
) error {
	if len(forwarders) > 0 {
		metadata["roomId"] = room.ID
		scheduler, err := schedulerCache.LoadScheduler(db, room.SchedulerName, true)
		if err != nil {
			return err
		}
		metadata["game"] = scheduler.Game
		var config *models.ConfigYAML
		if schedulerCache != nil {
			config, err = schedulerCache.LoadConfigYaml(db, room.SchedulerName, true)
		} else {
			config, err = models.NewConfigYAML(scheduler.YAML)
		}
		if err != nil {
			return err
		}
		if len(config.Forwarders) > 0 {
			enabledForwarders := getEnabledForwarders(config.Forwarders, forwarders)
			if len(enabledForwarders) > 0 {
				return ForwardEventToForwarders(enabledForwarders, event, metadata)
			}
		}
	}
	return nil
}

// ForwardEventToForwarders forwards
func ForwardEventToForwarders(forwarders []EventForwarder, event string, infos map[string]interface{}) error {
	for _, f := range forwarders {
		_, err := f.Forward(event, infos)
		if err != nil {
			return err
		}
	}
	return nil
}
