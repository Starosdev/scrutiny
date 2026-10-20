package database

import (
	"context"
	"errors"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/golang/mock/gomock"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_DownsampleScript_Weekly(t *testing.T) {
	t.Parallel()

	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()

	deviceRepo := scrutinyRepository{
		appConfig: fakeConfig,
	}

	aggregationType := "weekly"

	//test
	influxDbScript := deviceRepo.DownsampleScript(aggregationType, "tsk-weekly-aggr", "0 1 * * 0")

	//assert
	require.Equal(t, `
option task = { 
  name: "tsk-weekly-aggr",
  cron: "0 1 * * 0",
}

sourceBucket = "metrics"
rangeStart = -2w
rangeEnd = -1w
aggWindow = 1w
destBucket = "metrics_weekly"
destOrg = "scrutiny"

from(bucket: sourceBucket)
|> range(start: rangeStart, stop: rangeEnd)
|> filter(fn: (r) => r["_measurement"] == "smart" )
|> group(columns: ["device_id", "device_wwn", "_field"])
|> aggregateWindow(every: aggWindow, fn: last, createEmpty: false)
|> to(bucket: destBucket, org: destOrg)

from(bucket: sourceBucket)
|> range(start: rangeStart, stop: rangeEnd)
|> filter(fn: (r) => r["_measurement"] == "temp")
|> group(columns: ["device_id", "device_wwn"])
|> toInt()
|> aggregateWindow(fn: mean, every: aggWindow, createEmpty: false)
|> set(key: "_measurement", value: "temp")
|> set(key: "_field", value: "temp")
|> to(bucket: destBucket, org: destOrg)
		`, influxDbScript)
}

func Test_DownsampleScript_Monthly(t *testing.T) {
	t.Parallel()

	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()

	deviceRepo := scrutinyRepository{
		appConfig: fakeConfig,
	}

	aggregationType := "monthly"

	//test
	influxDbScript := deviceRepo.DownsampleScript(aggregationType, "tsk-monthly-aggr", "30 1 1 * *")

	//assert
	require.Equal(t, `
option task = { 
  name: "tsk-monthly-aggr",
  cron: "30 1 1 * *",
}

sourceBucket = "metrics_weekly"
rangeStart = -2mo
rangeEnd = -1mo
aggWindow = 1mo
destBucket = "metrics_monthly"
destOrg = "scrutiny"

from(bucket: sourceBucket)
|> range(start: rangeStart, stop: rangeEnd)
|> filter(fn: (r) => r["_measurement"] == "smart" )
|> group(columns: ["device_id", "device_wwn", "_field"])
|> aggregateWindow(every: aggWindow, fn: last, createEmpty: false)
|> to(bucket: destBucket, org: destOrg)

from(bucket: sourceBucket)
|> range(start: rangeStart, stop: rangeEnd)
|> filter(fn: (r) => r["_measurement"] == "temp")
|> group(columns: ["device_id", "device_wwn"])
|> toInt()
|> aggregateWindow(fn: mean, every: aggWindow, createEmpty: false)
|> set(key: "_measurement", value: "temp")
|> set(key: "_field", value: "temp")
|> to(bucket: destBucket, org: destOrg)
		`, influxDbScript)
}

func Test_DownsampleScript_Yearly(t *testing.T) {
	t.Parallel()

	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()

	deviceRepo := scrutinyRepository{
		appConfig: fakeConfig,
	}

	aggregationType := "yearly"

	//test
	influxDbScript := deviceRepo.DownsampleScript(aggregationType, "tsk-yearly-aggr", "0 2 1 1 *")

	//assert
	require.Equal(t, `
option task = { 
  name: "tsk-yearly-aggr",
  cron: "0 2 1 1 *",
}

sourceBucket = "metrics_monthly"
rangeStart = -2y
rangeEnd = -1y
aggWindow = 1y
destBucket = "metrics_yearly"
destOrg = "scrutiny"

from(bucket: sourceBucket)
|> range(start: rangeStart, stop: rangeEnd)
|> filter(fn: (r) => r["_measurement"] == "smart" )
|> group(columns: ["device_id", "device_wwn", "_field"])
|> aggregateWindow(every: aggWindow, fn: last, createEmpty: false)
|> to(bucket: destBucket, org: destOrg)

from(bucket: sourceBucket)
|> range(start: rangeStart, stop: rangeEnd)
|> filter(fn: (r) => r["_measurement"] == "temp")
|> group(columns: ["device_id", "device_wwn"])
|> toInt()
|> aggregateWindow(fn: mean, every: aggWindow, createEmpty: false)
|> set(key: "_measurement", value: "temp")
|> set(key: "_field", value: "temp")
|> to(bucket: destBucket, org: destOrg)
		`, influxDbScript)
}

// stubTasksAPI replays a scripted task list, records what it was asked for, and applies writes
// to its own state so a second lookup sees them. Only the four methods ensureDownsampleTask
// calls are implemented; the embedded interface is nil, so any other call panics rather than
// returning a wrong answer quietly.
type stubTasksAPI struct {
	influxapi.TasksAPI

	tasks     []domain.Task
	findErr   error
	deleteErr error
	// createRace adds a second task on create, as a process that raced this one would have
	createRace bool

	filters []*influxapi.TaskFilter
	creates int
	updated []domain.Task
	deleted []string
}

var _ influxapi.TasksAPI = (*stubTasksAPI)(nil)

func (s *stubTasksAPI) FindTasks(_ context.Context, filter *influxapi.TaskFilter) ([]domain.Task, error) {
	s.filters = append(s.filters, filter)
	if s.findErr != nil {
		return nil, s.findErr
	}
	return append([]domain.Task(nil), s.tasks...), nil
}

func (s *stubTasksAPI) CreateTaskByFlux(_ context.Context, flux, _ string) (*domain.Task, error) {
	s.creates++
	created := domain.Task{Id: "task-created", Name: "tsk-weekly-aggr", Flux: flux}
	s.tasks = append(s.tasks, created)
	if s.createRace {
		s.tasks = append(s.tasks, domain.Task{Id: "task-racer", Name: "tsk-weekly-aggr", Flux: flux})
	}
	return &created, nil
}

func (s *stubTasksAPI) UpdateTask(_ context.Context, task *domain.Task) (*domain.Task, error) {
	s.updated = append(s.updated, *task)
	for i := range s.tasks {
		if s.tasks[i].Id == task.Id {
			s.tasks[i].Flux = task.Flux
		}
	}
	return task, nil
}

func (s *stubTasksAPI) DeleteTaskWithID(_ context.Context, taskID string) error {
	s.deleted = append(s.deleted, taskID)
	if s.deleteErr != nil {
		return s.deleteErr
	}
	remaining := make([]domain.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		if task.Id != taskID {
			remaining = append(remaining, task)
		}
	}
	s.tasks = remaining
	return nil
}

func newTaskTestRepository(tasks influxapi.TasksAPI) *scrutinyRepository {
	logger, _ := logrustest.NewNullLogger()
	return &scrutinyRepository{logger: logger, influxTaskApi: tasks}
}

// task names are not unique in InfluxDB, so two processes starting together can both create the
// same downsample task, and every copy is scheduled and writes to the same destination bucket.
// One task survives, with the current script, and the keeper is the lowest ID so that two
// processes pruning at once converge instead of deleting each other's.
func TestEnsureDownsampleTaskUpdatesKeeperAndPrunesDuplicates(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{tasks: []domain.Task{
		{Id: "task-2", Name: "tsk-weekly-aggr", Flux: "old script"},
		{Id: "task-1", Name: "tsk-weekly-aggr", Flux: "old script"},
	}}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr", "new script", "weekly")

	require.NoError(t, err)
	require.Equal(t, 0, tasks.creates)
	require.Len(t, tasks.updated, 1)
	require.Equal(t, "task-1", tasks.updated[0].Id)
	require.Equal(t, "new script", tasks.updated[0].Flux)
	require.Equal(t, []string{"task-2"}, tasks.deleted)
	require.Len(t, tasks.tasks, 1)
}

// losing the create race leaves two tasks behind. They are pruned on the spot rather than left
// scheduled until some later startup.
func TestEnsureDownsampleTaskPrunesDuplicateFromCreateRace(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{createRace: true}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr", "new script", "weekly")

	require.NoError(t, err)
	require.Equal(t, 1, tasks.creates)
	require.Empty(t, tasks.updated)
	require.Len(t, tasks.deleted, 1)
	require.Len(t, tasks.tasks, 1)
}

// the other process pruning the same duplicate gets here first, and a duplicate that is already
// gone is the wanted end state, so the delete failure does not fail startup.
func TestEnsureDownsampleTaskToleratesDeleteFailure(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{
		deleteErr: errors.New("not found: task not found"),
		tasks: []domain.Task{
			{Id: "task-1", Name: "tsk-weekly-aggr", Flux: "new script"},
			{Id: "task-2", Name: "tsk-weekly-aggr", Flux: "new script"},
		},
	}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr", "new script", "weekly")

	require.NoError(t, err)
	require.Equal(t, []string{"task-2"}, tasks.deleted)
}

// the steady state: one task with the current script is left entirely alone.
func TestEnsureDownsampleTaskLeavesCurrentTaskAlone(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{tasks: []domain.Task{
		{Id: "task-1", Name: "tsk-weekly-aggr", Flux: "new script"},
	}}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr", "new script", "weekly")

	require.NoError(t, err)
	require.Equal(t, 0, tasks.creates)
	require.Empty(t, tasks.updated)
	require.Empty(t, tasks.deleted)
}

// the lookup is scoped to this deployment's organization. An unscoped name query matches across
// every organization the token can read, and updating or deleting another organization's task
// would be worse than leaving a duplicate.
func TestEnsureDownsampleTaskScopesLookupToOrg(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr", "new script", "weekly")

	require.NoError(t, err)
	require.NotEmpty(t, tasks.filters)
	for _, filter := range tasks.filters {
		require.Equal(t, "org-scrutiny", filter.OrgID)
		require.Equal(t, "tsk-weekly-aggr", filter.Name)
	}
	require.Equal(t, 1, tasks.creates)
}

// a lookup failure leaves the task alone rather than failing startup, since InfluxDB is already
// proven reachable by this point and the next start retries. It must not pass silently, and it
// must not be read as "no task exists" and turned into a create.
func TestEnsureDownsampleTaskWarnsAndSkipsWhenLookupFails(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{findErr: errors.New("influxdb unreachable")}
	logger, hook := logrustest.NewNullLogger()
	repo := &scrutinyRepository{logger: logger, influxTaskApi: tasks}

	err := repo.ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr", "new script", "weekly")

	require.NoError(t, err)
	require.Equal(t, 0, tasks.creates)
	require.Empty(t, tasks.updated)
	require.NotNil(t, hook.LastEntry())
	require.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	require.Contains(t, hook.LastEntry().Message, "influxdb unreachable")
}
