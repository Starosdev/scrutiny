package zfs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	collectorConfig "github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/analogj/scrutiny/collector/pkg/zfs/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterPoolsSendsCompleteInventoryWithHostID(t *testing.T) {
	config, err := collectorConfig.Create()
	require.NoError(t, err)
	config.Set("host.id", "host-a")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/zfs/pools/register", r.URL.Path)
		var request models.ZFSPoolWrapper
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, "host-a", request.HostID)
		assert.True(t, request.Complete)
		assert.Empty(t, request.Data)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"host_id":"host-a","complete":true,"data":[]}`))
	}))
	defer server.Close()

	collector, err := CreateCollector(config, logrus.NewEntry(logrus.New()), server.URL+"/")
	require.NoError(t, err)

	response, err := collector.RegisterPools([]models.ZFSPool{})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.Success)
}

func TestRunSendsEmptyInventoryWhenNoPoolsAreAvailable(t *testing.T) {
	dir := t.TempDir()
	zpoolPath := filepath.Join(dir, "zpool")
	script := "#!/bin/sh\nprintf 'no pools available\\n' >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(zpoolPath, []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	config, err := collectorConfig.Create()
	require.NoError(t, err)
	config.Set("host.id", "host-a")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/zfs/pools/register", r.URL.Path)
		var request models.ZFSPoolWrapper
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, "host-a", request.HostID)
		assert.True(t, request.Complete)
		assert.Empty(t, request.Data)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"host_id":"host-a","complete":true,"data":[]}`))
	}))
	defer server.Close()

	collector, err := CreateCollector(config, logrus.NewEntry(logrus.New()), server.URL+"/")
	require.NoError(t, err)
	require.NoError(t, collector.Run())
}

func TestRegisterPoolsUsesLegacyEnvelopeWithoutHostID(t *testing.T) {
	config, err := collectorConfig.Create()
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request models.ZFSPoolWrapper
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Empty(t, request.HostID)
		assert.False(t, request.Complete)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer server.Close()

	collector, err := CreateCollector(config, logrus.NewEntry(logrus.New()), server.URL+"/")
	require.NoError(t, err)
	_, err = collector.RegisterPools(nil)
	require.NoError(t, err)
}
