package testing

import (
	"fmt"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis"
	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	redismocks "github.com/topfreegames/extensions/redis/mocks"
	yaml "gopkg.in/yaml.v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
	metricsapi "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	fakeMetricsClient "k8s.io/metrics/pkg/client/clientset_generated/clientset/fake"

	"github.com/golang/mock/gomock"
	"github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/pg"
	"github.com/topfreegames/maestro/controller"
	"github.com/topfreegames/maestro/mocks"
	"github.com/topfreegames/maestro/models"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
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

	allMetrics := []string{
		string(models.CPUAutoScalingPolicyType),
		string(models.MemAutoScalingPolicyType),
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
		for _, mt := range allMetrics {
			calls.Add(
				mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(room.SchedulerName, mt), room.ID))
		}
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

	err = MockSetScallingAmount(mockRedisClient, mockPipeline, mockDb, clientset, &configYaml, 0, yamlStr)
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

// MockSetScallingAmount mocks the call to adjust the scaling amount based on min and max limits
func MockSetScallingAmount(
	mockRedis *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	mockDb *pgmocks.MockDB,
	clientset kubernetes.Interface,
	configYaml *models.ConfigYAML,
	currrentRooms int,
	yamlString string,
) error {
	mockRedis.EXPECT().TxPipeline().Return(mockPipeline)

	creating := models.GetRoomStatusSetRedisKey(configYaml.Name, "creating")
	ready := models.GetRoomStatusSetRedisKey(configYaml.Name, "ready")
	occupied := models.GetRoomStatusSetRedisKey(configYaml.Name, "occupied")
	terminating := models.GetRoomStatusSetRedisKey(configYaml.Name, "terminating")

	mockPipeline.EXPECT().SCard(creating).Return(goredis.NewIntResult(int64(0), nil))
	mockPipeline.EXPECT().SCard(ready).Return(goredis.NewIntResult(int64(currrentRooms), nil))
	mockPipeline.EXPECT().SCard(occupied).Return(goredis.NewIntResult(int64(0), nil))
	mockPipeline.EXPECT().SCard(terminating).Return(goredis.NewIntResult(int64(0), nil))
	mockPipeline.EXPECT().Exec()

	return nil
}

// MockSetScallingAmountWithRoomStatusCount mocks the call to adjust the scaling amount based on min and max limits
func MockSetScallingAmountWithRoomStatusCount(
	mockRedis *redismocks.MockRedisClient,
	mockPipeline *redismocks.MockPipeliner,
	configYaml *models.ConfigYAML,
	expC *models.RoomsStatusCount,
) error {
	mockRedis.EXPECT().TxPipeline().Return(mockPipeline)

	creating := models.GetRoomStatusSetRedisKey(configYaml.Name, "creating")
	ready := models.GetRoomStatusSetRedisKey(configYaml.Name, "ready")
	occupied := models.GetRoomStatusSetRedisKey(configYaml.Name, "occupied")
	terminating := models.GetRoomStatusSetRedisKey(configYaml.Name, "terminating")

	mockPipeline.EXPECT().SCard(creating).Return(goredis.NewIntResult(int64(expC.Creating), nil))
	mockPipeline.EXPECT().SCard(ready).Return(goredis.NewIntResult(int64(expC.Ready), nil))
	mockPipeline.EXPECT().SCard(occupied).Return(goredis.NewIntResult(int64(expC.Occupied), nil))
	mockPipeline.EXPECT().SCard(terminating).Return(goredis.NewIntResult(int64(expC.Terminating), nil))
	mockPipeline.EXPECT().Exec()

	return nil
}

// MockRoomDistribution mocks the existence of rooms with various status (creating, ready, occupied and terminating)
func MockRoomDistribution(
	configYaml *models.ConfigYAML,
	mockPipeline *redismocks.MockPipeliner,
	mockRedisClient *redismocks.MockRedisClient,
	expC *models.RoomsStatusCount,
) {
	creating := models.GetRoomStatusSetRedisKey(configYaml.Name, "creating")
	ready := models.GetRoomStatusSetRedisKey(configYaml.Name, "ready")
	occupied := models.GetRoomStatusSetRedisKey(configYaml.Name, "occupied")
	terminating := models.GetRoomStatusSetRedisKey(configYaml.Name, "terminating")
	mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
	mockPipeline.EXPECT().SCard(creating).Return(goredis.NewIntResult(int64(expC.Creating), nil))
	mockPipeline.EXPECT().SCard(ready).Return(goredis.NewIntResult(int64(expC.Ready), nil))
	mockPipeline.EXPECT().SCard(occupied).Return(goredis.NewIntResult(int64(expC.Occupied), nil))
	mockPipeline.EXPECT().SCard(terminating).Return(goredis.NewIntResult(int64(expC.Terminating), nil))
	mockPipeline.EXPECT().Exec()
}

// MockSendUsage mocks SendUsage method. This method sends current usage percentage to redis set
func MockSendUsage(mockPipeline *redismocks.MockPipeliner, mockRedisClient *redismocks.MockRedisClient, autoScaling *models.AutoScaling) {
	metricSent := map[string]bool{}

	for _, trigger := range autoScaling.Up.MetricsTrigger {
		metricSent[string(trigger.Type)] = true
		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		mockPipeline.EXPECT().LPush(gomock.Any(), gomock.Any())
		mockPipeline.EXPECT().LTrim(gomock.Any(), gomock.Any(), gomock.Any())
		mockPipeline.EXPECT().Exec()
	}

	for _, trigger := range autoScaling.Down.MetricsTrigger {
		if !metricSent[string(trigger.Type)] {
			mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
			mockPipeline.EXPECT().LPush(gomock.Any(), gomock.Any())
			mockPipeline.EXPECT().LTrim(gomock.Any(), gomock.Any(), gomock.Any())
			mockPipeline.EXPECT().Exec()
		}
	}
}

// MockGetUsages mockes the return of usage percentages from redis
func MockGetUsages(
	mockPipeline *redismocks.MockPipeliner,
	mockRedisClient *redismocks.MockRedisClient,
	key string,
	size, usage, percentageOfPointsGreaterThanUsage, times int,
) {
	mid := size * percentageOfPointsGreaterThanUsage / 100
	usages := make([]string, size)
	for idx := range usages {
		if idx < mid {
			usages[idx] = strconv.FormatFloat(float64(usage)/100+0.1, 'f', 1, 32)
		} else {
			usages[idx] = strconv.FormatFloat(float64(usage)/100-0.1, 'f', 1, 32)
		}
	}
	mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(times)
	mockPipeline.EXPECT().LRange(key, gomock.Any(), gomock.Any()).Return(goredis.NewStringSliceResult(
		usages, nil,
	)).Times(times)
	mockPipeline.EXPECT().Exec().Times(times)
}

// MockGetScheduler mocks the retrieval of a scheduler
func MockGetScheduler(
	mockDb *pgmocks.MockDB,
	configYaml *models.ConfigYAML,
	state, yamlString string,
	lastChangedAt, lastScaleOpAt time.Time,
	times int,
) {
	// Mock scheduler
	mockDb.EXPECT().Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", configYaml.Name).Do(func(scheduler *models.Scheduler, query string, modifier string) {
		scheduler.State = state
		scheduler.StateLastChangedAt = lastChangedAt.Unix()
		scheduler.LastScaleOpAt = lastScaleOpAt.Unix()
		scheduler.YAML = yamlString
	}).Times(times)
}

// MockRedisReadyPop mocks removal from redis ready set
func MockRedisReadyPop(
	mockPipeline *redismocks.MockPipeliner,
	mockRedisClient *redismocks.MockRedisClient,
	schedulerName string,
	amount int,
) {
	readyKey := models.GetRoomStatusSetRedisKey(schedulerName, models.StatusReady)
	for i := 0; i < amount; i++ {
		mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
		mockPipeline.EXPECT().SPop(readyKey).Return(goredis.NewStringResult(fmt.Sprintf("room-%d", i), nil))
		mockPipeline.EXPECT().Exec()
	}
}

// MockClearAll mocks models.Room.ClearAll method
func MockClearAll(
	mockPipeline *redismocks.MockPipeliner,
	mockRedisClient *redismocks.MockRedisClient,
	schedulerName string,
	amount int,
) {
	allStatus := []string{
		models.StatusCreating,
		models.StatusReady,
		models.StatusOccupied,
		models.StatusTerminating,
		models.StatusTerminated,
	}
	allMetrics := []string{
		string(models.CPUAutoScalingPolicyType),
		string(models.MemAutoScalingPolicyType),
	}

	mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline)
	mockPipeline.EXPECT().Exec()

	for i := 0; i < amount; i++ {
		room := models.NewRoom(fmt.Sprintf("room-%d", i), schedulerName)

		for _, status := range allStatus {
			mockPipeline.EXPECT().
				SRem(models.GetRoomStatusSetRedisKey(schedulerName, status), room.GetRoomRedisKey())
			mockPipeline.EXPECT().
				ZRem(models.GetLastStatusRedisKey(schedulerName, status), room.ID)
		}
		mockPipeline.EXPECT().ZRem(models.GetRoomPingRedisKey(schedulerName), room.ID)
		for _, mt := range allMetrics {
			mockPipeline.EXPECT().ZRem(models.GetRoomMetricsRedisKey(schedulerName, mt), room.ID)
		}
		mockPipeline.EXPECT().Del(room.GetRoomRedisKey())
	}
}

// MockScaleUp mocks all Scale Up operations on redis
func MockScaleUp(
	mockPipeline *redismocks.MockPipeliner,
	mockRedisClient *redismocks.MockRedisClient,
	schedulerName string,
	times int,
) {
	mockRedisClient.EXPECT().TxPipeline().Return(mockPipeline).Times(times)
	mockPipeline.EXPECT().HMSet(gomock.Any(), gomock.Any()).Do(
		func(schedulerName string, statusInfo map[string]interface{}) {
			gomega.Expect(statusInfo["status"]).To(gomega.Equal("creating"))
			gomega.Expect(statusInfo["lastPing"]).To(gomega.BeNumerically("~", time.Now().Unix(), 1))
		},
	).Times(times)
	mockPipeline.EXPECT().ZAdd(models.GetRoomPingRedisKey(schedulerName), gomock.Any()).Times(times)
	mockPipeline.EXPECT().SAdd(models.GetRoomStatusSetRedisKey(schedulerName, "creating"), gomock.Any()).Times(times)
	mockPipeline.EXPECT().Exec().Times(times)
}

// CopyAutoScaling copies an autoscaling struct to a new one
func CopyAutoScaling(original, clone *models.AutoScaling) {
	clone.Up = &models.ScalingPolicy{}
	clone.Up.MetricsTrigger = original.Up.MetricsTrigger
	if original.Up.Trigger != nil {
		clone.Up.Cooldown = original.Up.Cooldown
		clone.Up.Delta = original.Up.Delta
		clone.Up.Trigger = &models.ScalingPolicyTrigger{}
		clone.Up.Trigger.Limit = original.Up.Trigger.Limit
		clone.Up.Trigger.Threshold = original.Up.Trigger.Threshold
		clone.Up.Trigger.Time = original.Up.Trigger.Time
		clone.Up.Trigger.Usage = original.Up.Trigger.Usage
	}

	clone.Down = &models.ScalingPolicy{}
	clone.Down.MetricsTrigger = original.Down.MetricsTrigger
	if original.Down.Trigger != nil {
		clone.Down.Cooldown = original.Down.Cooldown
		clone.Down.Delta = original.Down.Delta
		clone.Down.Trigger = &models.ScalingPolicyTrigger{}
		clone.Down.Trigger.Limit = original.Down.Trigger.Limit
		clone.Down.Trigger.Threshold = original.Down.Trigger.Threshold
		clone.Down.Trigger.Time = original.Down.Trigger.Time
		clone.Down.Trigger.Usage = original.Down.Trigger.Usage
	}
}

// TransformLegacyInMetricsTrigger maps legacy to metrics trigger
func TransformLegacyInMetricsTrigger(autoScalingInfo *models.AutoScaling) {
	// Up
	if len(autoScalingInfo.Up.MetricsTrigger) == 0 {
		autoScalingInfo.Up.MetricsTrigger = append(
			autoScalingInfo.Up.MetricsTrigger,
			&models.ScalingPolicyMetricsTrigger{
				Type:      models.LegacyAutoScalingPolicyType,
				Usage:     autoScalingInfo.Up.Trigger.Usage,
				Limit:     autoScalingInfo.Up.Trigger.Limit,
				Threshold: autoScalingInfo.Up.Trigger.Threshold,
				Time:      autoScalingInfo.Up.Trigger.Time,
				Delta:     autoScalingInfo.Up.Delta,
			},
		)
	}
	// Down
	if len(autoScalingInfo.Down.MetricsTrigger) == 0 {
		autoScalingInfo.Down.MetricsTrigger = append(
			autoScalingInfo.Down.MetricsTrigger,
			&models.ScalingPolicyMetricsTrigger{
				Type:      models.LegacyAutoScalingPolicyType,
				Usage:     autoScalingInfo.Down.Trigger.Usage,
				Limit:     autoScalingInfo.Down.Trigger.Limit,
				Threshold: autoScalingInfo.Down.Trigger.Threshold,
				Time:      autoScalingInfo.Down.Trigger.Time,
				Delta:     -autoScalingInfo.Down.Delta,
			},
		)
	}
}

// ContainerMetricsDefinition is a struct that stores the container metrics values to use on MockCPUAndMemoryMetricsClient
type ContainerMetricsDefinition struct {
	Usage    map[models.AutoScalingPolicyType]int
	MemScale resource.Scale
	Name     string
}

// BuildContainerMetricsArray build an array of container metrics to use on MockCPUAndMemoryMetricsClient
func BuildContainerMetricsArray(containerDefinitions []ContainerMetricsDefinition) []metricsapi.ContainerMetrics {
	var containerMetricsArr []metricsapi.ContainerMetrics
	for _, container := range containerDefinitions {
		containerMetricsArr = append(
			containerMetricsArr,
			metricsapi.ContainerMetrics{
				Name: container.Name,
				Usage: v1.ResourceList{
					v1.ResourceCPU: *resource.NewMilliQuantity(
						int64(container.Usage[models.CPUAutoScalingPolicyType]),
						resource.DecimalSI),
					v1.ResourceMemory: *resource.NewScaledQuantity(
						int64(container.Usage[models.MemAutoScalingPolicyType]), container.MemScale),
				},
			},
		)
	}
	return containerMetricsArr
}

// CreatePod mocks create pod method setting cpu and mem requests
func CreatePod(clientset *fake.Clientset, cpuRequests, memRequests, schedulerName, podName, containerName string) {
	pod := &v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: containerName,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse(cpuRequests),
							v1.ResourceMemory: resource.MustParse(memRequests),
						},
					},
				},
			},
		},
	}
	if podName != "" {
		pod.SetName(podName)
	} else {
		pod.SetName(schedulerName)
	}
	clientset.CoreV1().Pods(schedulerName).Create(pod)
}

// CreatePodMetricsList returns a fakeMetricsClientset with reactor to PodMetricses Get call
func CreatePodMetricsList(containers []metricsapi.ContainerMetrics, schedulerName string, errArray ...error) *fakeMetricsClient.Clientset {
	myFakeMetricsClient := &fakeMetricsClient.Clientset{}

	myFakeMetricsClient.AddReactor("get", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
		if len(errArray) > 0 && errArray[0] != nil {
			return true, nil, errArray[0]
		}
		podMetric := &metricsapi.PodMetrics{
			ObjectMeta: metav1.ObjectMeta{
				Name:      schedulerName,
				Namespace: schedulerName,
			},
			Timestamp:  metav1.Time{Time: time.Now()},
			Window:     metav1.Duration{Duration: time.Minute},
			Containers: containers,
		}
		return true, podMetric, nil
	})

	return myFakeMetricsClient
}

// CreatePodsMetricsList returns a fakeMetricsClientset with reactor to PodMetricses List call
// It will use the same array of containers for every pod
func CreatePodsMetricsList(containers []metricsapi.ContainerMetrics, pods []string, schedulerName string, errArray ...error) *fakeMetricsClient.Clientset {
	myFakeMetricsClient := &fakeMetricsClient.Clientset{}

	myFakeMetricsClient.AddReactor("list", "pods", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
		if len(errArray) > 0 && errArray[0] != nil {
			return true, nil, errArray[0]
		}
		metrics := &metricsapi.PodMetricsList{}
		for _, pod := range pods {
			podMetric := metricsapi.PodMetrics{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pod,
					Namespace: schedulerName,
				},
				Timestamp:  metav1.Time{Time: time.Now()},
				Window:     metav1.Duration{Duration: time.Minute},
				Containers: containers,
			}
			metrics.Items = append(metrics.Items, podMetric)
		}
		return true, metrics, nil
	})

	return myFakeMetricsClient
}
