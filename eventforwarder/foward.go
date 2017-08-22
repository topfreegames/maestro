package eventforwarder

import (
	"github.com/topfreegames/extensions/pg/interfaces"
	"github.com/topfreegames/maestro/models"
	"k8s.io/client-go/kubernetes"
)

// ForwardRoomEvent forwards room event to app eventforwarders
func ForwardRoomEvent(
	forwarders []EventForwarder,
	db interfaces.DB,
	kubernetesClient kubernetes.Interface,
	room *models.Room,
	status string,
	metadata map[string]interface{},
) error {
	if len(forwarders) > 0 {
		infos, err := room.GetRoomInfos(db, kubernetesClient)
		if err != nil {
			return err
		}
		infos["metadata"] = metadata
		return ForwardEventToForwarders(forwarders, status, infos)
	}
	return nil
}

// ForwardPlayerEvent forwards player event to app eventforwarders
func ForwardPlayerEvent(
	forwarders []EventForwarder,
	db interfaces.DB,
	roomID string,
	event string,
	metadata map[string]interface{},
) error {
	if len(forwarders) > 0 {
		metadata["roomId"] = roomID
		return ForwardEventToForwarders(forwarders, event, metadata)
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
