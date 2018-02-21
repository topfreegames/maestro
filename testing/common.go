package testing

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"

	goredis "github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/models"
	yaml "gopkg.in/yaml.v2"
)

// MockSelectScheduler selects a scheduler on database
func MockSelectScheduler(
	yamlStr string,
	mockDb *pgmocks.MockDB,
) (calls []*gomock.Call) {
	calls = []*gomock.Call{}

	var configYaml models.ConfigYAML
	yaml.Unmarshal([]byte(yamlStr), &configYaml)

	calls = append(calls,
		mockDb.EXPECT().
			Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
			Do(func(scheduler *models.Scheduler, query string, modifier string) {
				*scheduler = *models.NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
			}))

	return calls
}

// MockRedisLock mocks a lock creation on redis
func MockRedisLock(
	mockRedisClient *redismocks.MockRedisClient,
	lockKey string,
	lockTimeoutMs int,
	lockResult bool,
	errLock error,
) (calls []*gomock.Call) {
	calls = []*gomock.Call{}

	calls = append(calls,
		mockRedisClient.EXPECT().
			SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
			Return(goredis.NewBoolResult(lockResult, errLock)))

	return calls
}

// MockReturnRedisLock mocks the script that returns the lock
func MockReturnRedisLock(
	mockRedisClient *redismocks.MockRedisClient,
	lockKey string,
	errLock error,
) {
	mockRedisClient.EXPECT().Ping().AnyTimes()
	mockRedisClient.EXPECT().
		Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
		Return(goredis.NewCmdResult(nil, errLock))
}

// MockUpdateSchedulerOnDB mocks the update call to DB
func MockUpdateSchedulerOnDB(
	mockDb *pgmocks.MockDB,
) {
	mockDb.EXPECT().
		Query(gomock.Any(), "UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id", gomock.Any())
}

// MockRemoveRoomsFromRedis mocks the room creation from pod
func MockRemoveRoomsFromRedis(
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	pods *v1.PodList,
	configYaml *models.ConfigYAML,
) {
	allStatus := []string{
		models.StatusCreating,
		models.StatusReady,
		models.StatusOccupied,
		models.StatusTerminating,
		models.StatusTerminated,
	}

	for _, pod := range pods.Items {
		// Remove room from redis
		mockRedisClient.EXPECT().
			TxPipeline().
			Return(mockPipeline)
		room := models.NewRoom(pod.GetName(), pod.GetNamespace())
		for _, status := range allStatus {
			mockPipeline.EXPECT().
				SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey())
			mockPipeline.EXPECT().
				ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID)
		}
		mockPipeline.EXPECT().
			ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID)
		mockPipeline.EXPECT().
			Del(room.GetRoomRedisKey())
		mockPipeline.EXPECT().
			Exec()

			// Retrieve ports to pool
		if configYaml.Version() == "v1" {
			mockRedisClient.EXPECT().
				TxPipeline().
				Return(mockPipeline)
			for range configYaml.Ports {
				mockPipeline.EXPECT().
					SAdd(models.FreePortsRedisKey(), gomock.Any())
			}
			mockPipeline.EXPECT().
				Exec()
		} else if configYaml.Version() == "v2" {
			for _, c := range configYaml.Containers {
				mockRedisClient.EXPECT().
					TxPipeline().
					Return(mockPipeline)
				for range c.Ports {
					mockPipeline.EXPECT().
						SAdd(models.FreePortsRedisKey(), gomock.Any())
				}
				mockPipeline.EXPECT().
					Exec()
			}
		}
	}
}

// MockCreateRoomsFromPods mocks the
func MockCreateRoomsFromPods(
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	configYaml *models.ConfigYAML,
) {
	for i := 0; i < configYaml.AutoScaling.Min; i++ {
		mockRedisClient.EXPECT().
			TxPipeline().
			Return(mockPipeline)

		mockPipeline.EXPECT().
			HMSet(gomock.Any(), gomock.Any()).
			Do(func(schedulerName string, statusInfo map[string]interface{}) {
				gomega.Expect(statusInfo["status"]).
					To(gomega.Equal(models.StatusCreating))
				gomega.Expect(statusInfo["lastPing"]).
					To(gomega.BeNumerically("~", time.Now().Unix(), 1))
			})

		mockPipeline.EXPECT().
			ZAdd(models.GetRoomPingRedisKey(configYaml.Name), gomock.Any())

		mockPipeline.EXPECT().
			SAdd(models.GetRoomStatusSetRedisKey(configYaml.Name, "creating"),
				gomock.Any())

		mockPipeline.EXPECT().Exec()

		if configYaml.Version() == "v1" {
			mockRedisClient.EXPECT().
				TxPipeline().
				Return(mockPipeline)

			for range configYaml.Ports {
				mockPipeline.EXPECT().
					SPop(models.FreePortsRedisKey()).
					Return(goredis.NewStringResult("5000", nil))
			}

			mockPipeline.EXPECT().
				Exec()
		} else if configYaml.Version() == "v2" {
			for _, c := range configYaml.Containers {
				mockRedisClient.EXPECT().
					TxPipeline().
					Return(mockPipeline)

				for range c.Ports {
					mockPipeline.EXPECT().
						SPop(models.FreePortsRedisKey()).
						Return(goredis.NewStringResult("5000", nil))
				}

				mockPipeline.EXPECT().
					Exec()
			}
		}
	}
}

// MockCreateScheduler mocks the creation of a scheduler
func MockCreateScheduler(
	clientset kubernetes.Interface,
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	mockDb *pgmocks.MockDB,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
	yamlStr string,
	timeoutSec int,
) {
	var configYaml models.ConfigYAML
	err := yaml.Unmarshal([]byte(yamlStr), &configYaml)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	mockRedisClient.EXPECT().
		TxPipeline().
		Return(mockPipeline).
		Times(configYaml.AutoScaling.Min)
	mockPipeline.EXPECT().
		HMSet(gomock.Any(), gomock.Any()).Do(
		func(schedulerName string, statusInfo map[string]interface{}) {
			gomega.Expect(statusInfo["status"]).To(gomega.Equal(models.StatusCreating))
			gomega.Expect(statusInfo["lastPing"]).To(gomega.BeNumerically("~", time.Now().Unix(), 1))
		},
	).Times(configYaml.AutoScaling.Min)

	mockPipeline.EXPECT().
		ZAdd(models.GetRoomPingRedisKey(configYaml.Name), gomock.Any()).
		Times(configYaml.AutoScaling.Min)
	mockPipeline.EXPECT().
		SAdd(models.GetRoomStatusSetRedisKey(configYaml.Name, "creating"), gomock.Any()).
		Times(configYaml.AutoScaling.Min)
	mockPipeline.EXPECT().
		Exec().
		Times(configYaml.AutoScaling.Min)

	mockDb.EXPECT().Query(
		gomock.Any(),
		"INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at) VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at) RETURNING id",
		gomock.Any(),
	)
	mockDb.EXPECT().Query(
		gomock.Any(),
		"UPDATE schedulers SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at) WHERE id=?id",
		gomock.Any(),
	)

	for _, c := range configYaml.Containers {
		mockRedisClient.EXPECT().
			TxPipeline().
			Return(mockPipeline).
			Times(configYaml.AutoScaling.Min)
		mockPipeline.EXPECT().
			SPop(models.FreePortsRedisKey()).
			Return(goredis.NewStringResult("5000", nil)).
			Times(configYaml.AutoScaling.Min * len(c.Ports))
		mockPipeline.EXPECT().
			Exec().
			Times(configYaml.AutoScaling.Min)
	}

	err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml, timeoutSec)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}
