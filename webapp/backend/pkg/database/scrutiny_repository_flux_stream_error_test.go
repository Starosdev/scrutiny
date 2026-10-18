package database

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/stretchr/testify/require"
)

// streamErrorQuery returns a result whose error surfaces while the stream is read, which is
// how a Flux runtime error reaches the caller. Query() itself reports request-level failures
// only, so a reader that checks only its error treats a failed query as an empty one.
type streamErrorQuery struct{ stubQueryAPI }

func (s *streamErrorQuery) Query(_ context.Context, _ string) (*influxapi.QueryTableResult, error) {
	return influxapi.NewQueryTableResult(io.NopCloser(strings.NewReader("invalid,csv\n"))), nil
}

// fixes #995: an empty slice and a nil error made a failed query indistinguishable from a
// device that never reported, which showed as "0 visible, 0 hidden" on the detail page.
func TestSmartAttributeHistoryPropagatesStreamErrors(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	require.NoError(t, repo.gormClient.Create(&models.Device{
		DeviceID: "device-1",
		WWN:      "wwn-1",
	}).Error)
	repo.influxQueryApi = &streamErrorQuery{}

	history, err := repo.GetSmartAttributeHistory(context.Background(), "device-1", DURATION_KEY_WEEK, 0, 0, nil)

	require.Error(t, err)
	require.Nil(t, history)
}

// fixes #995: the dashboard counterpart. Returning the summaries anyway showed every device
// as having no SMART data rather than reporting that the query failed.
func TestGetSummaryPropagatesStreamErrors(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	require.NoError(t, repo.gormClient.Create(&models.Device{
		DeviceID: "device-1",
		WWN:      "wwn-1",
	}).Error)
	repo.influxQueryApi = &streamErrorQuery{}

	summaries, err := repo.getSummary(context.Background(), false)

	require.Error(t, err)
	require.Nil(t, summaries)
}
