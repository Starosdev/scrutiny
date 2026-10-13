package notify

import (
	"context"
	"testing"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestIsScriptURL(t *testing.T) {
	require.True(t, IsScriptURL("script:///usr/local/bin/hook.sh"))
	require.True(t, IsScriptURL("  SCRIPT://x"))
	require.False(t, IsScriptURL("https://example.com/script://"))
	require.False(t, IsScriptURL("slack://token"))
}

func dbUrlsFixture() []models.NotifyUrl {
	return []models.NotifyUrl{
		{ID: 1, URL: "slack://token", HeartbeatEnabled: true},
		{ID: 2, URL: "script:///bin/sh", HeartbeatEnabled: true},
		{ID: 3, URL: "Script:///bin/true"},
	}
}

func TestLoadDatabaseUrls_SkipsScriptUrlsWithWarning(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(mockCtrl)
	repo.EXPECT().GetNotifyUrls(gomock.Any()).Return(dbUrlsFixture(), nil)
	logger, hook := logtest.NewNullLogger()
	n := Notify{Logger: logger}

	n.LoadDatabaseUrls(context.Background(), repo)

	require.Equal(t, []string{"slack://token"}, n.DatabaseUrls)
	require.Len(t, hook.AllEntries(), 2)
	require.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	require.Contains(t, hook.LastEntry().Message, "notify.urls")
}

func TestLoadHeartbeatDatabaseUrls_SkipsScriptUrls(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(mockCtrl)
	repo.EXPECT().GetNotifyUrls(gomock.Any()).Return(dbUrlsFixture(), nil)
	logger, _ := logtest.NewNullLogger()
	n := Notify{Logger: logger}

	n.LoadHeartbeatDatabaseUrls(context.Background(), repo)

	require.Equal(t, []string{"slack://token"}, n.DatabaseUrls)
}

func TestExcludeScriptUrls_KeepsOrderOfRemaining(t *testing.T) {
	logger, _ := logtest.NewNullLogger()
	n := Notify{Logger: logger}
	require.Equal(t, []string{"a://1", "b://2"}, n.excludeScriptUrls([]string{"a://1", "script:///x", "b://2"}))
}
