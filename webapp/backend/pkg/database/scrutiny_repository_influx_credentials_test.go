package database

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/analogj/scrutiny/pkg/secrets"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// fakeInflux serves the two endpoints first-time setup touches and records the onboarding request.
type fakeInflux struct {
	mu        sync.Mutex
	allowed   bool
	setupBody map[string]interface{}
}

func (f *fakeInflux) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]bool{"allowed": f.allowed})
	case http.MethodPost:
		_ = json.NewDecoder(r.Body).Decode(&f.setupBody)
		f.allowed = false
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}
}

func newInfluxCredentialConfig(t *testing.T, srv *httptest.Server, dir, token, password string) *mock_config.MockInterface {
	t.Helper()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	cfg := mock_config.NewMockInterface(ctrl)
	cfg.EXPECT().GetString("web.influxdb.scheme").Return("http").AnyTimes()
	cfg.EXPECT().GetString("web.influxdb.host").Return(u.Hostname()).AnyTimes()
	cfg.EXPECT().GetString("web.influxdb.port").Return(u.Port()).AnyTimes()
	cfg.EXPECT().GetBool("web.influxdb.tls.insecure_skip_verify").Return(false).AnyTimes()
	cfg.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()
	cfg.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	cfg.EXPECT().GetString("web.influxdb.init_username").Return("admin").AnyTimes()
	cfg.EXPECT().GetString("web.influxdb.token").Return(token).AnyTimes()
	cfg.EXPECT().GetString("web.influxdb.init_password").Return(password).AnyTimes()
	cfg.EXPECT().GetString("web.secrets.dir").Return(dir).AnyTimes()
	return cfg
}

func Test_setupInfluxClient_FreshInstallGeneratesCredentials(t *testing.T) {
	fake := &fakeInflux{allowed: true}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	dir := t.TempDir()
	logger, hook := logrustest.NewNullLogger()

	cfg := newInfluxCredentialConfig(t, srv, dir, "", "")
	client, err := setupInfluxClient(context.Background(), cfg, logger)
	require.NoError(t, err)
	client.Close()

	savedToken, err := secrets.Read(dir, secrets.InfluxTokenFile)
	require.NoError(t, err)
	savedPassword, err := secrets.Read(dir, secrets.InfluxPasswordFile)
	require.NoError(t, err)
	require.Len(t, savedToken, 64)
	require.NotEqual(t, LegacyInfluxToken, savedToken)
	require.NotEqual(t, "password12345", savedPassword)

	require.Equal(t, savedToken, fake.setupBody["token"])
	require.Equal(t, savedPassword, fake.setupBody["password"])

	info, err := os.Stat(filepath.Join(dir, secrets.InfluxTokenFile))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	for _, entry := range hook.AllEntries() {
		require.NotContains(t, entry.Message, savedToken)
		require.NotContains(t, entry.Message, savedPassword)
	}

	// restart: InfluxDB is now onboarded, the saved token is used and no warning is logged.
	hook.Reset()
	token, err := resolveInfluxToken(cfg, logger, true)
	require.NoError(t, err)
	require.Equal(t, savedToken, token)
	require.Empty(t, hook.AllEntries())
}

func Test_setupInfluxClient_FreshInstallHonorsConfiguredToken(t *testing.T) {
	fake := &fakeInflux{allowed: true}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	dir := t.TempDir()
	logger, _ := logrustest.NewNullLogger()

	client, err := setupInfluxClient(context.Background(), newInfluxCredentialConfig(t, srv, dir, "operator-token", "operator-pass"), logger)
	require.NoError(t, err)
	client.Close()

	require.Equal(t, "operator-token", fake.setupBody["token"])
	require.Equal(t, "operator-pass", fake.setupBody["password"])
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "configured credentials must not be written to disk")
}

func Test_setupInfluxClient_OnboardedInstanceSkipsSetup(t *testing.T) {
	fake := &fakeInflux{allowed: false}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	logger, _ := logrustest.NewNullLogger()

	client, err := setupInfluxClient(context.Background(), newInfluxCredentialConfig(t, srv, t.TempDir(), "", ""), logger)
	require.NoError(t, err)
	client.Close()
	require.Nil(t, fake.setupBody, "an onboarded InfluxDB must not be set up again")
}

func Test_resolveInfluxToken_LegacyInstallKeepsDefaultToken(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	logger, hook := logrustest.NewNullLogger()

	token, err := resolveInfluxToken(newInfluxCredentialConfig(t, srv, t.TempDir(), "", ""), logger, true)
	require.NoError(t, err)
	require.Equal(t, LegacyInfluxToken, token)
	require.Len(t, hook.AllEntries(), 1)
	require.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	require.Contains(t, hook.LastEntry().Message, "Rotate")
	require.NotContains(t, hook.LastEntry().Message, LegacyInfluxToken)
}

func Test_resolveInfluxToken_ExplicitLegacyTokenWarns(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	logger, hook := logrustest.NewNullLogger()

	token, err := resolveInfluxToken(newInfluxCredentialConfig(t, srv, t.TempDir(), LegacyInfluxToken, ""), logger, true)
	require.NoError(t, err)
	require.Equal(t, LegacyInfluxToken, token)
	require.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
}

func Test_resolveInfluxToken_ConfiguredTokenOnOnboardedInstance(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	logger, hook := logrustest.NewNullLogger()

	token, err := resolveInfluxToken(newInfluxCredentialConfig(t, srv, t.TempDir(), "my-token", ""), logger, true)
	require.NoError(t, err)
	require.Equal(t, "my-token", token)
	require.Empty(t, hook.AllEntries())
}

func Test_resolveInfluxToken_FreshInstallUnwritableDirFails(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	logger, _ := logrustest.NewNullLogger()
	// a regular file where the directory should be makes MkdirAll fail regardless of the user
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))

	_, err := resolveInfluxToken(newInfluxCredentialConfig(t, srv, filepath.Join(blocker, "config"), "", ""), logger, false)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "web.influxdb.token"))
}
