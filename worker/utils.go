package worker

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Sirupsen/logrus"
	"github.com/topfreegames/maestro/models"
)

func retrievePorts(
	w *Worker,
	start, end int,
	isGlobal bool,
	schedulerNames ...string,
) (err error) {
	log := w.Logger.WithFields(logrus.Fields{
		"operation": "RetrieveFreePorts",
		"function":  "retrievePorts",
	})

	portsPoolKey := models.FreePortsRedisKey()
	name := "global"
	if !isGlobal {
		name = schedulerNames[0]
		portsPoolKey = models.FreeSchedulerPortsRedisKey(name)
	}

	log = log.WithFields(logrus.Fields{
		"pool":       name,
		"startRange": start,
		"endRange":   end,
	})

	log.Info("starting retrieve ports to pool")

	// Make all changes in another Set then Rename it. In case of error, redis does not rollback.
	redisKey := "maestro:updated:free:ports"

	tx := w.RedisClient.Client.TxPipeline()
	for i := start; i <= end; i++ {
		tx.SAdd(redisKey, i)
	}

	for _, schedulerName := range schedulerNames {
		pods, err := w.KubernetesClient.CoreV1().Pods(schedulerName).List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, pod := range pods.Items {
			for _, port := range pod.Spec.Containers[0].Ports {
				tx.SRem(redisKey, port.HostPort)
			}
		}
	}

	tx.Rename(redisKey, portsPoolKey)
	_, err = tx.Exec()
	if err != nil {
		log.WithError(err).Error("failed to update ports pool")
		return err
	}

	log.Info("successfully retrieved ports to pool")

	return nil
}
