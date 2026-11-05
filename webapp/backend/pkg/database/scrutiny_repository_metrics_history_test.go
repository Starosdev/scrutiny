package database

import (
	"context"
	"errors"
	"strings"
	"testing"

	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/stretchr/testify/require"
)

// recordingQueryAPI captures the Flux query text so a test can assert on it without an
// InfluxDB server. It fails the query so the repository function returns before it has to
// parse a result.
type recordingQueryAPI struct {
	queries []string
}

var errRecordingQuery = errors.New("recording query api: not executed")

func (r *recordingQueryAPI) QueryRaw(ctx context.Context, query string, dialect *domain.Dialect) (string, error) {
	return "", errRecordingQuery
}

func (r *recordingQueryAPI) QueryRawWithParams(ctx context.Context, query string, dialect *domain.Dialect, params interface{}) (string, error) {
	return "", errRecordingQuery
}

func (r *recordingQueryAPI) Query(ctx context.Context, query string) (*influxapi.QueryTableResult, error) {
	r.queries = append(r.queries, query)
	return nil, errRecordingQuery
}

func (r *recordingQueryAPI) QueryWithParams(ctx context.Context, query string, params interface{}) (*influxapi.QueryTableResult, error) {
	r.queries = append(r.queries, query)
	return nil, errRecordingQuery
}

var _ influxapi.QueryAPI = (*recordingQueryAPI)(nil)

func createMetricsHistoryRepository(t *testing.T) (*scrutinyRepository, *recordingQueryAPI) {
	t.Helper()
	repo := createDeviceSelfTestRepository(t)
	queryAPI := &recordingQueryAPI{}
	repo.influxQueryApi = queryAPI
	return repo, queryAPI
}

// Flux query parameters (params.*) are InfluxDB Cloud only; InfluxDB OSS rejects them with
// "undefined identifier params" (#157, #1111). The identifier must be inlined as a quoted
// Flux string literal.
func TestGetBtrfsMetricsHistoryInlinesValidatedUUID(t *testing.T) {
	repo, queryAPI := createMetricsHistoryRepository(t)
	const uuid = "0f3c6a4e-1b2d-4c5e-8f9a-0b1c2d3e4f5a"

	_, err := repo.GetBtrfsMetricsHistory(context.Background(), uuid, DURATION_KEY_WEEK)
	require.ErrorContains(t, err, errRecordingQuery.Error())

	require.Len(t, queryAPI.queries, 1)
	query := queryAPI.queries[0]
	require.Contains(t, query, `r["filesystem_uuid"] == "`+uuid+`"`)
	require.NotContains(t, query, "params.")
}

func TestGetBtrfsMetricsHistoryRejectsInvalidUUIDBeforeQuerying(t *testing.T) {
	repo, queryAPI := createMetricsHistoryRepository(t)

	_, err := repo.GetBtrfsMetricsHistory(context.Background(), `x") or true or ("`, DURATION_KEY_WEEK)
	require.Error(t, err)
	require.Empty(t, queryAPI.queries)
}

func TestGetZFSPoolMetricsHistoryInlinesValidatedGUID(t *testing.T) {
	repo, queryAPI := createMetricsHistoryRepository(t)

	for _, guid := range []string{"12345678901234567890", "0xDEADBEEF"} {
		queryAPI.queries = nil

		_, err := repo.GetZFSPoolMetricsHistory(context.Background(), guid, DURATION_KEY_WEEK)
		require.ErrorContains(t, err, errRecordingQuery.Error())

		require.Len(t, queryAPI.queries, 1)
		query := queryAPI.queries[0]
		require.Contains(t, query, `r["pool_guid"] == "`+guid+`"`)
		require.NotContains(t, query, "params.")
	}
}

func TestGetZFSPoolMetricsHistoryRejectsInvalidGUIDBeforeQuerying(t *testing.T) {
	repo, queryAPI := createMetricsHistoryRepository(t)

	_, err := repo.GetZFSPoolMetricsHistory(context.Background(), `1") or true or ("`, DURATION_KEY_WEEK)
	require.Error(t, err)
	require.Empty(t, queryAPI.queries)
}

// Guard against the pattern creeping back in through a merge-conflict resolution, as
// happened to GetZFSPoolMetricsHistory after #157.
func TestMetricsHistoryQueriesNeverUseFluxParams(t *testing.T) {
	repo, queryAPI := createMetricsHistoryRepository(t)

	_, _ = repo.GetBtrfsMetricsHistory(context.Background(), "0f3c6a4e-1b2d-4c5e-8f9a-0b1c2d3e4f5a", DURATION_KEY_WEEK)
	_, _ = repo.GetZFSPoolMetricsHistory(context.Background(), "1", DURATION_KEY_WEEK)

	require.Len(t, queryAPI.queries, 2)
	for _, query := range queryAPI.queries {
		require.False(t, strings.Contains(query, "params."), "query uses Flux params: %s", query)
	}
}
