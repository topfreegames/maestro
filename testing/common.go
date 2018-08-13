package testing

import (
	"time"

	goredis "github.com/go-redis/redis"
	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	yaml "gopkg.in/yaml.v2"

	"github.com/golang/mock/gomock"
	"github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/pg"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/mocks"
	"github.com/topfreegames/maestro/models"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
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

func mockRemoveRoomsFromRedis(
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

// MockRemoveRoomStatusFromRedis removes room only from redis
func MockRemoveRoomStatusFromRedis(
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	pods *v1.PodList,
	configYaml *models.ConfigYAML,
) (calls *Calls) {
	return mockRemoveRoomsFromRedis(mockRedisClient, mockPipeline,
		pods, configYaml)
}

// MockRemoveRoomsFromRedis mocks the room creation from pod
func MockRemoveRoomsFromRedis(
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	pods *v1.PodList,
	configYaml *models.ConfigYAML,
) (calls *Calls) {
	return mockRemoveRoomsFromRedis(mockRedisClient, mockPipeline,
		pods, configYaml)
}

func mockCreateRooms(
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
	}

	return calls
}

// MockCreateRooms mocks the creation of rooms on redis
func MockCreateRooms(
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	configYaml *models.ConfigYAML,
) (calls *Calls) {
	return mockCreateRooms(mockRedisClient, mockPipeline, configYaml)
}

// MockCreateRoomsWithPorts mocks the creation of rooms on redis when
// scheduler has port range
func MockCreateRoomsWithPorts(
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	configYaml *models.ConfigYAML,
) (calls *Calls) {
	return mockCreateRooms(mockRedisClient, mockPipeline, configYaml)
}

// MockCreateScheduler mocks the creation of a scheduler
func MockCreateScheduler(
	clientset kubernetes.Interface,
	mockRedisClient *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	mockDb *pgmocks.MockDB,
	logger logrus.FieldLogger,
	roomManager models.RoomManager,
	mr *models.MixedMetricsReporter,
	yamlStr string,
	timeoutSec int,
	mockPortChooser *mocks.MockPortChooser,
	workerPortRange string,
	portStart, portEnd int,
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
		MockGetPortsFromPool(&configYaml, mockRedisClient, mockPortChooser, workerPortRange, portStart, portEnd))

	calls.Append(
		MockUpdateSchedulerStatus(mockDb, nil, nil))

	err = controller.CreateScheduler(logger, roomManager, mr, mockDb, mockRedisClient, clientset, &configYaml, timeoutSec)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return calls
}

// MockGetPortsFromPool mocks the function that chooses random ports
// to be used as HostPort in the pods
func MockGetPortsFromPool(
	configYaml *models.ConfigYAML,
	mockRedisClient *redismocks.MockRedisClient,
	mockPortChooser *mocks.MockPortChooser,
	workerPortRange string,
	portStart, portEnd int,
) (calls *Calls) {
	calls = NewCalls()

	if !configYaml.HasPorts() {
		return
	}

	if !configYaml.PortRange.IsSet() {
		mockRedisClient.EXPECT().
			Get(models.GlobalPortsPoolKey).
			Return(goredis.NewStringResult(workerPortRange, nil)).
			Times(configYaml.AutoScaling.Min)
	}

	if mockPortChooser == nil {
		return
	}

	givePorts := func(nPorts int) {
		ports := make([]int, nPorts)
		for i := 0; i < nPorts; i++ {
			ports[i] = portStart + i
		}
		mockPortChooser.EXPECT().
			Choose(portStart, portEnd, nPorts).
			Return(ports).
			Times(configYaml.AutoScaling.Min)
	}

	if configYaml.Version() == "v1" {
		givePorts(len(configYaml.Ports))
	} else if configYaml.Version() == "v2" {
		for _, container := range configYaml.Containers {
			givePorts(len(container.Ports))
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
				"SELECT version, created_at FROM scheduler_versions WHERE name = ? ORDER BY created_at ASC",
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

// MockSetDescription mocks the set description call
func MockSetDescription(
	opManager *models.OperationManager,
	mockRedisClient *redismocks.MockRedisClient,
	description string,
	err error,
) (calls *Calls) {
	calls = NewCalls()

	calls.Add(
		mockRedisClient.EXPECT().
			HMSet(opManager.GetOperationKey(), map[string]interface{}{
				"description": description,
			}).
			Return(goredis.NewStatusResult("", err)))

	return calls
}

// MockAnySetDescription mocks the set description call
func MockAnySetDescription(
	opManager *models.OperationManager,
	mockRedisClient *redismocks.MockRedisClient,
	description string,
	err error,
) (calls *Calls) {
	calls = NewCalls()

	calls.Add(
		mockRedisClient.EXPECT().
			HMSet(gomock.Any(), map[string]interface{}{
				"description": description,
			}).
			Return(goredis.NewStatusResult("", err)))

	return calls
}

// MockSelectSchedulerNames mocks the ListSchedulersNames function
func MockSelectSchedulerNames(
	mockDb *pgmocks.MockDB,
	schedulerNames []string,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	calls.Add(
		mockDb.EXPECT().
			Query(gomock.Any(), "SELECT name FROM schedulers").
			Do(func(schedulers *[]models.Scheduler, _ string) {
				*schedulers = make([]models.Scheduler, len(schedulerNames))
				for idx, name := range schedulerNames {
					(*schedulers)[idx] = models.Scheduler{Name: name}
				}
			}).
			Return(pg.NewTestResult(nil, 1), errDB))

	return
}

// MockSelectConfigYamls mocks the LoadSchedulers function
func MockSelectConfigYamls(
	mockDb *pgmocks.MockDB,
	schedulersToReturn []models.Scheduler,
	errDB error,
) (calls *Calls) {
	calls = NewCalls()

	calls.Add(
		mockDb.EXPECT().
			Query(gomock.Any(), "SELECT * FROM schedulers WHERE name IN (?)", gomock.Any()).
			Do(func(schedulers *[]models.Scheduler, _ string, _ ...interface{}) {
				*schedulers = schedulersToReturn
			}).
			Return(pg.NewTestResult(nil, 1), errDB))

	return
}

// MockPopulatePortPool mocks the InitAvailablePorts function
func MockPopulatePortPool(
	mockRedisClient *redismocks.MockRedisClient,
	freePortsKey string,
	begin, end int,
	err error,
) (calls *Calls) {
	calls = NewCalls()

	calls.Add(mockRedisClient.EXPECT().
		Eval(`
if redis.call("EXISTS", KEYS[1]) == 0 then
  for i=ARGV[1],ARGV[2] do
    redis.call("SADD", KEYS[1], i)
  end
end
return "OK"
`, []string{freePortsKey}, begin, end).
		Return(goredis.NewCmdResult(nil, err)))

	return
}

// MockGetRegisteredRooms mocks the call that gets all rooms on redis
func MockGetRegisteredRooms(
	mockRedis *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	schedulerName string,
	results [][]string,
	err error,
) {
	mockRedis.EXPECT().TxPipeline().Return(mockPipeline)
	for idx, status := range []string{
		models.StatusCreating, models.StatusReady,
		models.StatusOccupied, models.StatusTerminating,
	} {
		result := []string{}
		if idx < len(results) {
			result = results[idx]
		}
		key := models.GetRoomStatusSetRedisKey(schedulerName, status)
		mockPipeline.EXPECT().SMembers(key).Return(
			goredis.NewStringSliceResult(result, nil))
	}
	mockPipeline.EXPECT().Exec().Return(nil, err)
}
