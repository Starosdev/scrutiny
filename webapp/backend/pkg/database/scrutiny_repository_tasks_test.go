package database

import (
	"context"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/golang/mock/gomock"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/sirupsen/logrus"
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

// stubTasksAPI replays a scripted task list. Only the three methods ensureDownsampleTask calls
// are implemented; any other call panics rather than returning a wrong answer quietly.
type stubTasksAPI struct {
	influxapi.TasksAPI

	tasks   []domain.Task
	creates int
	updated []domain.Task
}

var _ influxapi.TasksAPI = (*stubTasksAPI)(nil)

func (s *stubTasksAPI) FindTasks(_ context.Context, _ *influxapi.TaskFilter) ([]domain.Task, error) {
	return s.tasks, nil
}

func (s *stubTasksAPI) CreateTaskByFlux(_ context.Context, flux, _ string) (*domain.Task, error) {
	s.creates++
	return &domain.Task{Flux: flux}, nil
}

func (s *stubTasksAPI) UpdateTask(_ context.Context, task *domain.Task) (*domain.Task, error) {
	s.updated = append(s.updated, *task)
	return task, nil
}

// task names are not unique in InfluxDB, so two processes starting together can both create the
// same downsample task. Treating that as a state to leave alone pinned the duplicates to the
// flux script they were first created with, because no later startup would update them.
func TestEnsureDownsampleTaskUpdatesScriptWhenDuplicatesExist(t *testing.T) {
	t.Parallel()

	tasks := &stubTasksAPI{tasks: []domain.Task{
		{Id: "task-1", Name: "tsk-weekly-aggr", Flux: "old script"},
		{Id: "task-2", Name: "tsk-weekly-aggr", Flux: "old script"},
	}}
	repo := &scrutinyRepository{logger: logrus.New(), influxTaskApi: tasks}

	err := repo.ensureDownsampleTask(context.Background(), "org-scrutiny", "tsk-weekly-aggr", "new script", "weekly")

	require.NoError(t, err)
	require.Equal(t, 0, tasks.creates)
	require.Len(t, tasks.updated, 1)
	require.Equal(t, "new script", tasks.updated[0].Flux)
}
