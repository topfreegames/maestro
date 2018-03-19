package testing

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"

	goredis "github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	"github.com/topfreegames/extensions/pg"
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
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	var configYaml models.ConfigYAML
	yaml.Unmarshal([]byte(yamlStr), &configYaml)

	calls.Add(
		mockDb.EXPECT().
			Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).
			Do(func(scheduler *models.Scheduler, query string, modifier string) {
				*scheduler = *models.NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
			}).
			Return(pg.NewTestResult(nil, 1), errDB))

	return calls
}

// MockRedisLock mocks a lock creation on redis
func MockRedisLock(
	mockRedisClient *redismocks.MockRedisClient,
	lockKey string,
	lockTimeoutMs int,
	lockResult bool,
	errLock error,
) (calls *Calls) {
	calls = NewCalls()

	calls.Add(mockRedisClient.EXPECT().
		SetNX(lockKey, gomock.Any(), time.Duration(lockTimeoutMs)*time.Millisecond).
		Return(goredis.NewBoolResult(lockResult, errLock)))

	return calls
}

// MockReturnRedisLock mocks the script that returns the lock
func MockReturnRedisLock(
	mockRedisClient *redismocks.MockRedisClient,
	lockKey string,
	errLock error,
) (calls *Calls) {
	calls = NewCalls()

	calls.Add(
		mockRedisClient.EXPECT().Ping().AnyTimes())

	calls.Add(
		mockRedisClient.EXPECT().
			Eval(gomock.Any(), []string{lockKey}, gomock.Any()).
			Return(goredis.NewCmdResult(nil, errLock)))

	return calls
}

// MockUpdateSchedulersTable mocks update on schedulers table
func MockUpdateSchedulersTable(
	mockDb *pgmocks.MockDB,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	query := "UPDATE schedulers SET (game, yaml, version) = (?game, ?yaml, ?version) WHERE id = ?id"
	calls.Add(
		mockDb.EXPECT().
			Query(gomock.Any(), query, gomock.Any()).
			Return(pg.NewTestResult(nil, 1), errDB))

	return calls
}

// MockInsertIntoVersionsTable mocks insert into scheduler_versions table
func MockInsertIntoVersionsTable(
	scheduler *models.Scheduler,
	mockDb *pgmocks.MockDB,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	query := `INSERT INTO scheduler_versions (name, version, yaml) 
	VALUES (?, ?, ?)`
	calls.Add(mockDb.EXPECT().
		Query(gomock.Any(), query, scheduler.Name, scheduler.Version, gomock.Any()).
		Return(pg.NewTestResult(nil, 1), errDB))

	return calls
}

// MockCountNumberOfVersions mocks the call to select how many versions
// are of a scheduler
func MockCountNumberOfVersions(
	scheduler *models.Scheduler,
	returnCount int,
	mockDb *pgmocks.MockDB,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	query := "SELECT COUNT(*) FROM scheduler_versions WHERE name = ?"
	calls.Add(
		mockDb.EXPECT().
			Query(gomock.Any(), query, scheduler.Name).
			Do(func(count *int, _ string, _ string) {
				*count = returnCount
			}).
			Return(pg.NewTestResult(nil, 1), errDB))

	return calls
}

// MockDeleteOldVersions mocks the deletions of old versions
func MockDeleteOldVersions(
	scheduler *models.Scheduler,
	deletedVersions int,
	mockDb *pgmocks.MockDB,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	query := `DELETE FROM scheduler_versions WHERE id IN (
			SELECT id
			FROM scheduler_versions
			WHERE name = ?
			ORDER BY created_at ASC 
			LIMIT ?
		)`
	calls.Add(
		mockDb.EXPECT().
			Exec(query, scheduler.Name, deletedVersions).
			Return(pg.NewTestResult(nil, 1), errDB))

	return calls
}

// MockRemoveRoomsFromRedis mocks the room creation from pod
func MockRemoveRoomsFromRedis(
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	pods *v1.PodList,
	configYaml *models.ConfigYAML,
) (calls *Calls) {
	calls = NewCalls()

	allStatus := []string{
		models.StatusCreating,
		models.StatusReady,
		models.StatusOccupied,
		models.StatusTerminating,
		models.StatusTerminated,
	}

	for _, pod := range pods.Items {
		// Retrieve ports to pool
		for _, c := range pod.Spec.Containers {
			if len(c.Ports) > 0 {
				calls.Add(
					mockRedisClient.EXPECT().
						TxPipeline().
						Return(mockPipeline))
				for range c.Ports {
					calls.Add(
						mockPipeline.EXPECT().
							SAdd(models.FreePortsRedisKey(), gomock.Any()))
				}
				calls.Add(
					mockPipeline.EXPECT().
						Exec())
			}
		}
		room := models.NewRoom(pod.GetName(), pod.GetNamespace())
		calls.Add(
			mockRedisClient.EXPECT().
				TxPipeline().
				Return(mockPipeline))
		for _, status := range allStatus {
			calls.Add(
				mockPipeline.EXPECT().
					SRem(models.GetRoomStatusSetRedisKey(room.SchedulerName, status), room.GetRoomRedisKey()))
			calls.Add(
				mockPipeline.EXPECT().
					ZRem(models.GetLastStatusRedisKey(room.SchedulerName, status), room.ID))
		}
		calls.Add(
			mockPipeline.EXPECT().
				ZRem(models.GetRoomPingRedisKey(pod.GetNamespace()), room.ID))
		calls.Add(
			mockPipeline.EXPECT().
				Del(room.GetRoomRedisKey()))
		calls.Add(
			mockPipeline.EXPECT().
				Exec())
	}

	return calls
}

// MockCreateRooms mocks the creation of rooms on redis
func MockCreateRooms(
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	configYaml *models.ConfigYAML,
) (calls *Calls) {
	calls = NewCalls()

	for i := 0; i < configYaml.AutoScaling.Min; i++ {
		calls.Add(
			mockRedisClient.EXPECT().
				TxPipeline().
				Return(mockPipeline))

		calls.Add(
			mockPipeline.EXPECT().
				HMSet(gomock.Any(), gomock.Any()).
				Do(func(schedulerName string, statusInfo map[string]interface{}) {
					gomega.Expect(statusInfo["status"]).
						To(gomega.Equal(models.StatusCreating))
					gomega.Expect(statusInfo["lastPing"]).
						To(gomega.BeNumerically("~", time.Now().Unix(), 1))
				}))

		calls.Add(
			mockPipeline.EXPECT().
				SAdd(models.GetRoomStatusSetRedisKey(configYaml.Name, "creating"),
					gomock.Any()))

		calls.Add(
			mockPipeline.EXPECT().
				ZAdd(models.GetRoomPingRedisKey(configYaml.Name), gomock.Any()))

		calls.Add(
			mockPipeline.EXPECT().Exec())

		if configYaml.Version() == "v1" {
			calls.Add(
				mockRedisClient.EXPECT().
					TxPipeline().
					Return(mockPipeline))

			for range configYaml.Ports {
				calls.Add(
					mockPipeline.EXPECT().
						SPop(models.FreePortsRedisKey()).
						Return(goredis.NewStringResult("5000", nil)))
			}

			calls.Add(
				mockPipeline.EXPECT().
					Exec())
		} else if configYaml.Version() == "v2" {
			for _, c := range configYaml.Containers {
				if len(c.Ports) > 0 {
					calls.Add(
						mockRedisClient.EXPECT().
							TxPipeline().
							Return(mockPipeline))

					for range c.Ports {
						calls.Add(
							mockPipeline.EXPECT().
								SPop(models.FreePortsRedisKey()).
								Return(goredis.NewStringResult("5000", nil)))
					}

					calls.Add(
						mockPipeline.EXPECT().
							Exec())
				}
			}
		}
	}

	return calls
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
) (calls *Calls) {
	calls = NewCalls()

	var configYaml models.ConfigYAML
	err := yaml.Unmarshal([]byte(yamlStr), &configYaml)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	calls.Append(
		MockInsertScheduler(mockDb, nil))

	calls.Add(
		mockRedisClient.EXPECT().
			TxPipeline().
			Return(mockPipeline).
			Times(configYaml.AutoScaling.Min))

	calls.Add(
		mockPipeline.EXPECT().
			HMSet(gomock.Any(), gomock.Any()).Do(
			func(schedulerName string, statusInfo map[string]interface{}) {
				gomega.Expect(statusInfo["status"]).To(gomega.Equal(models.StatusCreating))
				gomega.Expect(statusInfo["lastPing"]).To(gomega.BeNumerically("~", time.Now().Unix(), 1))
			},
		).Times(configYaml.AutoScaling.Min))

	calls.Add(
		mockPipeline.EXPECT().
			ZAdd(models.GetRoomPingRedisKey(configYaml.Name), gomock.Any()).
			Times(configYaml.AutoScaling.Min))
	calls.Add(
		mockPipeline.EXPECT().
			SAdd(models.GetRoomStatusSetRedisKey(configYaml.Name, "creating"), gomock.Any()).
			Times(configYaml.AutoScaling.Min))
	calls.Add(
		mockPipeline.EXPECT().
			Exec().
			Times(configYaml.AutoScaling.Min))

	calls.Append(
		MockGetPortsFromPool(&configYaml, mockRedisClient, mockPipeline))

	calls.Append(
		MockUpdateSchedulerStatus(mockDb, nil, nil))

	err = controller.CreateScheduler(logger, mr, mockDb, mockRedisClient, clientset, &configYaml, timeoutSec)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return calls
}

// MockGetPortsFromPool mocks the function that gets free ports from redis
// to be used as HostPort in the pods
func MockGetPortsFromPool(
	configYaml *models.ConfigYAML,
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
) (calls *Calls) {
	calls = NewCalls()

	if configYaml.Version() == "v1" {
		calls.Add(
			mockRedisClient.EXPECT().
				TxPipeline().
				Return(mockPipeline).
				Times(configYaml.AutoScaling.Min))
		calls.Add(
			mockPipeline.EXPECT().
				SPop(models.FreePortsRedisKey()).
				Return(goredis.NewStringResult("5000", nil)).
				Times(configYaml.AutoScaling.Min * len(configYaml.Ports)))
		calls.Add(
			mockPipeline.EXPECT().
				Exec().
				Times(configYaml.AutoScaling.Min))
	} else if configYaml.Version() == "v2" {
		for _, c := range configYaml.Containers {
			if len(c.Ports) > 0 {
				calls.Add(
					mockRedisClient.EXPECT().
						TxPipeline().
						Return(mockPipeline).
						Times(configYaml.AutoScaling.Min))
				calls.Add(
					mockPipeline.EXPECT().
						SPop(models.FreePortsRedisKey()).
						Return(goredis.NewStringResult("5000", nil)).
						Times(configYaml.AutoScaling.Min * len(c.Ports)))
				calls.Add(
					mockPipeline.EXPECT().
						Exec().
						Times(configYaml.AutoScaling.Min))
			}
		}
	}

	return calls
}

// MockInsertScheduler inserts a new scheduler into database
func MockInsertScheduler(
	mockDb *pgmocks.MockDB,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	calls.Add(
		mockDb.EXPECT().
			Query(gomock.Any(), `INSERT INTO schedulers (name, game, yaml, state, state_last_changed_at, version) 
	VALUES (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?version) 
	RETURNING id`, gomock.Any()).
			Return(pg.NewTestResult(nil, 1), errDB))

	return calls
}

// MockUpdateSchedulerStatus mocks the scheduler update query on database
func MockUpdateSchedulerStatus(
	mockDb *pgmocks.MockDB,
	errUpdate, errInsert error,
) (calls *Calls) {
	calls = NewCalls()

	calls.Add(
		mockDb.EXPECT().
			Query(gomock.Any(), `UPDATE schedulers
	SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at, version) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at, ?version)
	WHERE id=?id`, gomock.Any()).
			Return(pg.NewTestResult(nil, 1), errUpdate))

	if errUpdate == nil {
		calls.Add(
			mockDb.EXPECT().
				Query(gomock.Any(), `INSERT INTO scheduler_versions (name, version, yaml)
	VALUES (?name, ?version, ?yaml)
	ON CONFLICT DO NOTHING`, gomock.Any()).
				Return(pg.NewTestResult(nil, 1), errInsert))
	}

	return calls
}

// MockUpdateSchedulerStatusAndDo mocks the scheduler update query on database
func MockUpdateSchedulerStatusAndDo(
	do func(base *models.Scheduler, query string, scheduler *models.Scheduler),
	mockDb *pgmocks.MockDB,
	errUpdate, errInsert error,
) (calls *Calls) {
	calls = NewCalls()

	calls.Add(
		mockDb.EXPECT().
			Query(gomock.Any(), `UPDATE schedulers
	SET (name, game, yaml, state, state_last_changed_at, last_scale_op_at, version) = (?name, ?game, ?yaml, ?state, ?state_last_changed_at, ?last_scale_op_at, ?version)
	WHERE id=?id`, gomock.Any()).
			Return(pg.NewTestResult(nil, 1), errUpdate).
			Do(do))

	if errUpdate == nil {
		calls.Add(
			mockDb.EXPECT().
				Query(gomock.Any(), `INSERT INTO scheduler_versions (name, version, yaml)
	VALUES (?name, ?version, ?yaml)
	ON CONFLICT DO NOTHING`, gomock.Any()).
				Return(pg.NewTestResult(nil, 1), errInsert))
	}

	return calls
}

// MockSelectYaml mocks the select of yaml from database
func MockSelectYaml(
	yamlStr string,
	mockDb *pgmocks.MockDB,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	var configYaml models.ConfigYAML
	yaml.Unmarshal([]byte(yamlStr), &configYaml)

	calls.Add(
		mockDb.EXPECT().
			Query(gomock.Any(), "SELECT yaml FROM schedulers WHERE name = ?", configYaml.Name).
			Do(func(scheduler *models.Scheduler, query string, modifier string) {
				*scheduler = *models.NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
			}).
			Return(pg.NewTestResult(nil, 1), errDB))

	return calls
}

// MockSelectYamlWithVersion mocks the select of a yaml version
func MockSelectYamlWithVersion(
	yamlStr, version string,
	mockDb *pgmocks.MockDB,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	var configYaml models.ConfigYAML
	yaml.Unmarshal([]byte(yamlStr), &configYaml)

	calls.Add(
		mockDb.EXPECT().
			Query(
				gomock.Any(),
				"SELECT yaml FROM scheduler_versions WHERE name = ? AND version = ?",
				configYaml.Name, version).
			Do(func(scheduler *models.Scheduler, query, name, version string) {
				*scheduler = *models.NewScheduler(configYaml.Name, configYaml.Game, yamlStr)
			}).
			Return(pg.NewTestResult(nil, 1), errDB))

	return calls
}

// MockSelectSchedulerVersions mocks the select to list scheduler versions
func MockSelectSchedulerVersions(
	yamlStr string,
	versions []string,
	mockDb *pgmocks.MockDB,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	var configYaml models.ConfigYAML
	yaml.Unmarshal([]byte(yamlStr), &configYaml)

	calls.Add(
		mockDb.EXPECT().
			Query(
				gomock.Any(),
				"SELECT version, created_at FROM scheduler_versions WHERE name = ?",
				configYaml.Name).
			Do(func(rVersions *[]*models.SchedulerVersion, query string, name string) {
				*rVersions = make([]*models.SchedulerVersion, len(versions))
				for i, version := range versions {
					(*rVersions)[i] = &models.SchedulerVersion{Version: version}
				}
			}).
			Return(pg.NewTestResult(nil, 1), errDB))

	return calls
}

// MockSelectPreviousSchedulerVersion mocks the query that gets the scheduler before
// current one
func MockSelectPreviousSchedulerVersion(
	name, previousVersion, previousYaml string,
	mockDb *pgmocks.MockDB,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	scheduler := models.NewScheduler(name, "", "")

	calls.Add(
		mockDb.EXPECT().
			Query(gomock.Any(), `SELECT * 
	FROM scheduler_versions 
	WHERE created_at < ( 
		SELECT created_at 
		FROM scheduler_versions 
		WHERE name = ?name AND version = ?version
	) AND name = ?name
	ORDER BY created_at DESC 
	LIMIT 1`, gomock.Any()).
			Do(func(rScheduler *models.Scheduler, _ string, _ *models.Scheduler) {
				*rScheduler = *scheduler
				rScheduler.Version = previousVersion
				rScheduler.YAML = previousYaml
			}).
			Return(pg.NewTestResult(nil, 1), errDB))

	return calls
}

// MockOperationManagerStart mocks the start of operation
func MockOperationManagerStart(
	opManager *models.OperationManager,
	timeout time.Duration,
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
) {
	mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
	mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any())
	mockPipeline.EXPECT().Expire(gomock.Any(), timeout)
	mockPipeline.EXPECT().Set(opManager.BuildCurrOpKey(), gomock.Any(), timeout)
	mockPipeline.EXPECT().Exec()
}

// MockOperationManager mocks the redis operations of opManager
func MockOperationManager(
	opManager *models.OperationManager,
	timeout time.Duration,
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
) {
	MockGetCurrentOperationKey(opManager, mockRedisClient, nil)
	MockOperationManagerStart(opManager, timeout, mockRedisClient, mockPipeline)

	mockRedisClient.EXPECT().HGetAll(gomock.Any()).Return(
		goredis.NewStringStringMapResult(map[string]string{
			"not": "empty",
		}, nil)).AnyTimes()

	mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
	mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any())
	mockPipeline.EXPECT().Expire(gomock.Any(), 10*time.Minute)
	mockPipeline.EXPECT().Del(opManager.BuildCurrOpKey())
	mockPipeline.EXPECT().Exec().Do(func() {
		opManager.StopLoop()
	})
}

// MockDeleteRedisKey mocks a delete operation on redis
func MockDeleteRedisKey(
	opManager *models.OperationManager,
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	err error,
) {
	mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
	mockPipeline.EXPECT().Del(opManager.GetOperationKey())
	mockPipeline.EXPECT().Exec().Return(nil, err)
}

// MockGetCurrentOperationKey mocks get current operation on redis
func MockGetCurrentOperationKey(
	opManager *models.OperationManager,
	mockRedisClient *redismocks.MockRedisClient,
	err error,
) {
	mockRedisClient.EXPECT().
		Get(opManager.BuildCurrOpKey()).
		Return(goredis.NewStringResult("", err))
}
