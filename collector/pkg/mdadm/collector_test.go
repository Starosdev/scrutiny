package mdadm

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/analogj/scrutiny/collector/pkg/mdadm/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterValidArrays(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	arrays := []models.MDADMArray{
		{Name: "md0", UUID: "uuid-1"},
		{Name: "md1", UUID: ""},
		{Name: "md2", UUID: "uuid-1"},
		{Name: "md3", UUID: " uuid-3 "},
	}
	metrics := []models.MDADMMetrics{{State: "clean"}, {State: "bad"}, {State: "dup"}, {State: "ok"}}

	filteredArrays, filteredMetrics := filterValidArrays(logger, arrays, metrics)

	require.Len(t, filteredArrays, 2)
	require.Len(t, filteredMetrics, 2)
	assert.Equal(t, "uuid-1", filteredArrays[0].UUID)
	assert.Equal(t, "uuid-3", filteredArrays[1].UUID)
	assert.Equal(t, "clean", filteredMetrics[0].State)
	assert.Equal(t, "ok", filteredMetrics[1].State)
}

func TestRegisterArraysReturnsHTTPErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"errors":["boom"]}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	collector, err := CreateCollector(nil, logrus.NewEntry(logrus.New()), server.URL+"/")
	require.NoError(t, err)

	_, err = collector.RegisterArrays([]models.MDADMArray{{Name: "md0", UUID: "uuid-1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
	assert.Contains(t, err.Error(), "boom")
}

func TestRunUploadsMetricsForRegisteredArraysWhenRegistrationIsPartial(t *testing.T) {
	registerCalls := 0
	metricUUIDs := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/mdadm/arrays/register":
			registerCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"success":true,"errors":["array md1 (uuid-2) registration failed: duplicate"],"data":[{"uuid":"uuid-1","name":"md0","level":"raid1","devices":["/dev/sda","/dev/sdb"]},{"uuid":"uuid-3","name":"md2","level":"raid5","devices":["/dev/sdc","/dev/sdd"]}]}`)
		case "/api/mdadm/array/uuid-1/metrics":
			metricUUIDs = append(metricUUIDs, "uuid-1")
			w.WriteHeader(http.StatusOK)
		case "/api/mdadm/array/uuid-3/metrics":
			metricUUIDs = append(metricUUIDs, "uuid-3")
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	collector, err := CreateCollector(nil, logrus.NewEntry(logrus.New()), server.URL+"/")
	require.NoError(t, err)

	arrays := []models.MDADMArray{
		{Name: "md0", UUID: "uuid-1", Level: "raid1"},
		{Name: "md1", UUID: "uuid-2", Level: "raid1"},
		{Name: "md2", UUID: "uuid-3", Level: "raid5"},
	}
	metrics := []models.MDADMMetrics{{State: "clean"}, {State: "clean"}, {State: "degraded"}}

	filteredArrays, filteredMetrics := filterValidArrays(collector.logger, arrays, metrics)
	wrapper, err := collector.RegisterArrays(filteredArrays)
	require.NoError(t, err)
	require.Len(t, wrapper.Data, 2)

	registered := map[string]bool{}
	for _, array := range wrapper.Data {
		registered[array.UUID] = true
	}
	for i, array := range filteredArrays {
		if !registered[array.UUID] {
			continue
		}
		require.NoError(t, collector.UploadMetrics(array, filteredMetrics[i]))
	}

	assert.Equal(t, 1, registerCalls)
	assert.ElementsMatch(t, []string{"uuid-1", "uuid-3"}, metricUUIDs)
}
