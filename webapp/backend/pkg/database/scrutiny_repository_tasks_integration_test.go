package database

import (
	"context"
	"testing"

	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/stretchr/testify/require"
)

// ensureDownsampleTask decides whether a task is this deployment's by looking for the generated
// script's destBucket assignment in the task's stored flux, so the whole ownership gate rests on
// InfluxDB handing back the flux it was given. If a server reformatted the script, no task would
// ever look owned and every startup would create another one. Stubs cannot answer that, so this
// runs against the real InfluxDB the Test Backend job provides.
func TestDownsampleTaskFluxSurvivesRoundTripThroughInfluxDB(t *testing.T) {
	host := integrationInfluxHost(t)
	repo := newSharedWWNIntegrationRepository(t, host)
	ctx := context.Background()

	org, err := repo.influxClient.OrganizationsAPI().FindOrganizationByName(ctx, "scrutiny")
	require.NoError(t, err)

	// a name of its own, so this cannot disturb the tasks EnsureTasks manages
	taskName := "tsk-roundtrip-aggr"
	script := repo.DownsampleScript("weekly", taskName, "0 1 * * 0")
	marker := downsampleDestBucketLine(script)
	require.NotEmpty(t, marker)

	created, err := repo.influxTaskApi.CreateTaskByFlux(ctx, script, *org.Id)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = repo.influxTaskApi.DeleteTaskWithID(context.Background(), created.Id)
	})

	found, err := repo.influxTaskApi.FindTasks(ctx, &influxapi.TaskFilter{Name: taskName, OrgID: *org.Id})
	require.NoError(t, err)
	require.Len(t, found, 1)

	// what the ownership gate needs: the destBucket assignment comes back intact
	require.Contains(t, found[0].Flux, marker, "InfluxDB returned a flux script without the destBucket assignment the task ownership gate matches on")

	if found[0].Flux != script {
		// not a failure: the keeper is updated whenever the stored flux differs, which only
		// costs one extra update per task per start. Worth reporting, because it means the
		// update is unavoidable rather than a sign of a stale script.
		t.Logf("InfluxDB stored a flux script that differs from the one sent; the task update on startup is therefore unconditional")
	}
}
