package database

import (
	"context"
	"errors"
	"fmt"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/golang/mock/gomock"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"strings"
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
	matching := []domain.Task{}
	for _, task := range s.tasks {
		if filter == nil || filter.Name == "" || task.Name == filter.Name {
			matching = append(matching, task)
		}
	}
	return matching, nil
}

func (s *stubTasksAPI) CreateTaskByFlux(_ context.Context, flux, _ string) (*domain.Task, error) {
	s.creates++
	// the real API takes the task name from the script's option block
	name := fluxTaskName(flux)
	created := domain.Task{Id: fmt.Sprintf("task-created-%d", s.creates), Name: name, Flux: flux}
	s.tasks = append(s.tasks, created)
	if s.createRace {
		s.tasks = append(s.tasks, domain.Task{Id: fmt.Sprintf("task-racer-%d", s.creates), Name: name, Flux: flux})
	}
	return &created, nil
}

// fluxTaskName reads the name out of a generated script's option block.
func fluxTaskName(flux string) string {
	for _, line := range strings.Split(flux, "\n") {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, `name: "`); ok {
			return strings.TrimSuffix(strings.TrimSuffix(after, `",`), `"`)
		}
	}
	return ""
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

// downsampleTestScript is a stand-in for a generated downsample script: what matters to task
// ownership is the destBucket assignment, and what matters to the update is the rest changing.
func downsampleTestScript(destBucket, version string) string {
	return downsampleTestScriptNamed("tsk-weekly-aggr-metrics", destBucket, version)
}

func downsampleTestScriptNamed(taskName, destBucket, version string) string {
	return fmt.Sprintf("option task = {\n  name: \"%s\",\n}\n\nsourceBucket = \"metrics\"\ndestBucket = \"%s\"\n\n// %s\n", taskName, destBucket, version)
}

var (
	ourScript    = downsampleTestScript("metrics_weekly", "current")
	ourOldScript = downsampleTestScript("metrics_weekly", "previous")
	// a second deployment in this organization, configured with a different base bucket: same
	// task name, different destination. The name deliberately starts with this deployment's,
	// so matching on the bare bucket name rather than the quoted assignment would adopt it.
	foreignScript = downsampleTestScript("metrics2_weekly", "current")
)

// the generated script carries the assignment that task ownership is matched on. If
// DownsampleScript stops emitting it, ownership silently stops being decidable, so pin the two
// together.
func TestDownsampleScriptCarriesDestBucketAssignment(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()
	repo := scrutinyRepository{appConfig: fakeConfig}

	script := repo.DownsampleScript("weekly", "tsk-weekly-aggr", "0 1 * * 0")

	require.Equal(t, `destBucket = "metrics_weekly"`, downsampleDestBucketLine(script))
}

// task names are not unique in InfluxDB, so two processes starting together can both create the
// same downsample task, and every copy is scheduled and writes to the same destination bucket.
// One task survives, with the current script, and the keeper is the lowest ID so that two
// processes pruning at once converge instead of deleting each other's.
func TestEnsureDownsampleTaskUpdatesKeeperAndPrunesDuplicates(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{tasks: []domain.Task{
		{Id: "task-2", Name: "tsk-weekly-aggr-metrics", Flux: ourOldScript},
		{Id: "task-1", Name: "tsk-weekly-aggr-metrics", Flux: ourOldScript},
	}}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr-metrics", ourScript, "weekly")

	require.NoError(t, err)
	require.Equal(t, 0, tasks.creates)
	require.Len(t, tasks.updated, 1)
	require.Equal(t, "task-1", tasks.updated[0].Id)
	require.Equal(t, ourScript, tasks.updated[0].Flux)
	require.Equal(t, []string{"task-2"}, tasks.deleted)
	require.Len(t, tasks.tasks, 1)
}

// a second deployment sharing this organization writes to its own destination bucket under the
// same task name. Its task is neither rewritten nor deleted, and this deployment creates its
// own rather than adopting it.
func TestEnsureDownsampleTaskLeavesForeignTaskAlone(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{tasks: []domain.Task{
		{Id: "task-1", Name: "tsk-weekly-aggr-metrics", Flux: foreignScript},
	}}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr-metrics", ourScript, "weekly")

	require.NoError(t, err)
	require.Equal(t, 1, tasks.creates)
	require.Empty(t, tasks.updated)
	require.Empty(t, tasks.deleted)
	require.Len(t, tasks.tasks, 2)
	require.Equal(t, foreignScript, tasks.tasks[0].Flux)
}

// losing the create race leaves two tasks behind. They are pruned on the spot rather than left
// scheduled until some later startup.
func TestEnsureDownsampleTaskPrunesDuplicateFromCreateRace(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{createRace: true}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr-metrics", ourScript, "weekly")

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
			{Id: "task-1", Name: "tsk-weekly-aggr-metrics", Flux: ourScript},
			{Id: "task-2", Name: "tsk-weekly-aggr-metrics", Flux: ourScript},
		},
	}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr-metrics", ourScript, "weekly")

	require.NoError(t, err)
	require.Equal(t, []string{"task-2"}, tasks.deleted)
}

// the steady state: one task with the current script is left entirely alone.
func TestEnsureDownsampleTaskLeavesCurrentTaskAlone(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{tasks: []domain.Task{
		{Id: "task-1", Name: "tsk-weekly-aggr-metrics", Flux: ourScript},
	}}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr-metrics", ourScript, "weekly")

	require.NoError(t, err)
	require.Equal(t, 0, tasks.creates)
	require.Empty(t, tasks.updated)
	require.Empty(t, tasks.deleted)
}

// a generated script with no destBucket assignment is a template defect, not a deployment
// state. Ownership cannot be decided, so nothing is deleted on a guess.
func TestEnsureDownsampleTaskWithoutDestBucketLeavesDuplicatesAlone(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{tasks: []domain.Task{
		{Id: "task-1", Name: "tsk-weekly-aggr-metrics", Flux: "old script"},
		{Id: "task-2", Name: "tsk-weekly-aggr-metrics", Flux: "old script"},
	}}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr", "script without a destination", "weekly")

	require.NoError(t, err)
	require.Empty(t, tasks.deleted)
	require.Empty(t, tasks.updated)
	require.Equal(t, 0, tasks.creates)
}

// the lookup is scoped to this deployment's organization. An unscoped name query matches across
// every organization the token can read, and updating or deleting another organization's task
// would be worse than leaving a duplicate.
func TestEnsureDownsampleTaskScopesLookupToOrg(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{}

	err := newTaskTestRepository(tasks).ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr-metrics", ourScript, "weekly")

	require.NoError(t, err)
	require.NotEmpty(t, tasks.filters)
	for _, filter := range tasks.filters {
		require.Equal(t, "org-scrutiny", filter.OrgID)
		require.Equal(t, "tsk-weekly-aggr-metrics", filter.Name)
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

	err := repo.ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr-metrics", ourScript, "weekly")

	require.NoError(t, err)
	require.Equal(t, 0, tasks.creates)
	require.Empty(t, tasks.updated)
	require.NotNil(t, hook.LastEntry())
	require.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	require.Contains(t, hook.LastEntry().Message, "influxdb unreachable")
}

func newEnsureTasksRepository(t *testing.T, tasks influxapi.TasksAPI, bucket string) *scrutinyRepository {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return(bucket).AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()

	logger, _ := logrustest.NewNullLogger()
	return &scrutinyRepository{appConfig: fakeConfig, logger: logger, influxTaskApi: tasks}
}

// the task names carry the configured bucket, so two deployments sharing an organization with
// different buckets no longer own same-named tasks. The two test binaries in CI are the same
// case: one configures metrics, the other its own bucket.
func TestEnsureTasksQualifiesTaskNamesWithTheBucket(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{}
	repo := newEnsureTasksRepository(t, tasks, "metrics_web_test")

	require.NoError(t, repo.EnsureTasks(context.Background(), "org-scrutiny"))

	require.Equal(t, 3, tasks.creates)
	created := []string{}
	for _, task := range tasks.tasks {
		created = append(created, task.Name)
	}
	require.ElementsMatch(t, []string{
		"tsk-weekly-aggr-metrics_web_test",
		"tsk-monthly-aggr-metrics_web_test",
		"tsk-yearly-aggr-metrics_web_test",
	}, created)
}

// upgrading leaves the task this deployment created under the unqualified name behind. Keeping it
// would double every down-sampling write, so it goes once the qualified one is in place.
func TestEnsureTasksRemovesItsOwnLegacyTasks(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{tasks: []domain.Task{
		{Id: "legacy-weekly", Name: "tsk-weekly-aggr", Flux: downsampleTestScriptNamed("tsk-weekly-aggr", "metrics_weekly", "previous")},
	}}
	repo := newEnsureTasksRepository(t, tasks, "metrics")

	require.NoError(t, repo.EnsureTasks(context.Background(), "org-scrutiny"))

	require.Equal(t, []string{"legacy-weekly"}, tasks.deleted)
	require.Equal(t, 3, tasks.creates)
}

// a legacy task writing somewhere else belongs to a deployment that has not upgraded yet.
// Deleting it would stop that deployment's down-sampling.
func TestEnsureTasksLeavesAnotherDeploymentsLegacyTaskAlone(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{tasks: []domain.Task{
		{Id: "legacy-foreign", Name: "tsk-weekly-aggr", Flux: downsampleTestScriptNamed("tsk-weekly-aggr", "metrics2_weekly", "previous")},
	}}
	repo := newEnsureTasksRepository(t, tasks, "metrics")

	require.NoError(t, repo.EnsureTasks(context.Background(), "org-scrutiny"))

	require.Empty(t, tasks.deleted)
	require.Equal(t, 3, tasks.creates)
}
