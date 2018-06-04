package eventforwarder_test

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/topfreegames/maestro/eventforwarder"
	"github.com/topfreegames/maestro/models"
	"github.com/topfreegames/maestro/reporters"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"testing"

	pgmocks "github.com/topfreegames/extensions/pg/mocks"
	eventforwardermock "github.com/topfreegames/maestro/eventforwarder/mock"
	reportermock "github.com/topfreegames/maestro/reporters/mocks"
)

func TestEventforwarder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eventforwarder Suite")
}

var (
	hook               *test.Hook
	logger             *logrus.Logger
	mockCtrl           *gomock.Controller
	mockDB             *pgmocks.MockDB
	mockEventForwarder *eventforwardermock.MockEventForwarder
	mockForwarders     []*eventforwarder.Info
	mockReporter       *reportermock.MockReporter
	room               *models.Room
	clientset          *fake.Clientset
	cache              *models.SchedulerCache
	metadata           map[string]interface{}
	schedulerName      = "scheduler"
	gameName           = "game"
	roomName           = "room-id"
	nodeAddress        = "1.2.3.4"
	hostPort           = int32(50000)
	yaml               = `name: scheduler
game: game
forwarders:
  mockplugin:
    mockfwd:
      enabled: true
`
)

var _ = BeforeEach(func() {
	var err error
	logger, hook = test.NewNullLogger()
	logger.Level = logrus.DebugLevel

	mockCtrl = gomock.NewController(GinkgoT())
	mockEventForwarder = eventforwardermock.NewMockEventForwarder(mockCtrl)
	mockForwarders = []*eventforwarder.Info{
		&eventforwarder.Info{
			Plugin:    "mockplugin",
			Name:      "mockfwd",
			Forwarder: mockEventForwarder,
		},
	}

	r := reporters.GetInstance()
	mockReporter = reportermock.NewMockReporter(mockCtrl)
	r.SetReporter("mockReporter", mockReporter)

	clientset = fake.NewSimpleClientset()

	mockDB = pgmocks.NewMockDB(mockCtrl)

	cache = models.NewSchedulerCache(1*time.Second, 1*time.Second, logger)
	mockDB.EXPECT().
		Query(gomock.Any(), "SELECT * FROM schedulers WHERE name = ?", schedulerName).
		Do(func(scheduler *models.Scheduler, _ string, _ string) {
			*scheduler = *models.NewScheduler(schedulerName, gameName, yaml)
		})
	_, err = cache.LoadScheduler(mockDB, schedulerName, false)
	Expect(err).NotTo(HaveOccurred())

	err = models.NewNamespace(schedulerName).Create(clientset)
	Expect(err).NotTo(HaveOccurred())

	pod := &v1.Pod{}
	pod.SetName(roomName)
	pod.SetNamespace(schedulerName)
	pod.Spec.NodeName = "node"
	pod.Spec.Containers = []v1.Container{
		{Ports: []v1.ContainerPort{
			{Name: "port", HostPort: hostPort},
		}},
	}
	_, err = clientset.CoreV1().Pods(schedulerName).Create(pod)
	Expect(err).NotTo(HaveOccurred())

	node := &v1.Node{}
	node.Status.Addresses = []v1.NodeAddress{
		{Type: v1.NodeExternalDNS, Address: nodeAddress},
	}
	node.Name = "node"
	_, err = clientset.CoreV1().Nodes().Create(node)

	room = models.NewRoom(roomName, schedulerName)
})
