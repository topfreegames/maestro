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
	metadata map[string]string,
) error {
	if len(forwarders) > 0 {
		infos, err := room.GetRoomInfos(db, kubernetesClient)
		if err != nil {
			return err
		}
		infos["metadata"] = metadata
		ForwardEventToForwarders(forwarders, status, infos)
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
