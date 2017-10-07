package eventforwarder

import (
	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
)

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
		config, err := models.NewConfigYAML(scheduler.YAML)
		if err != nil {
			return err
		}
		infos, err := room.GetRoomInfos(db, kubernetesClient, schedulerCache, scheduler)
		if err != nil {
			return err
		}
		infos["metadata"] = metadata
		if len(config.Forwarders) > 0 {
			enabledForwarders := make([]EventForwarder, 0)
			for _, info := range forwarders {
				if fwds, ok := config.Forwarders[info.Plugin]; ok {
					if fwd, ok := fwds[info.Name]; ok {
						if fwd.Enabled {
							enabledForwarders = append(enabledForwarders, info.Forwarder)
						}
					}
				}
			}
			if len(enabledForwarders) > 0 {
				return ForwardEventToForwarders(enabledForwarders, status, infos)
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
		config, err := models.NewConfigYAML(scheduler.YAML)
		if err != nil {
			return err
		}
		if len(config.Forwarders) > 0 {
			enabledForwarders := make([]EventForwarder, 0)
			for _, info := range forwarders {
				if fwds, ok := config.Forwarders[info.Plugin]; ok {
					if fwd, ok := fwds[info.Name]; ok {
						if fwd.Enabled {
							enabledForwarders = append(enabledForwarders, info.Forwarder)
						}
					}
				}
			}
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
