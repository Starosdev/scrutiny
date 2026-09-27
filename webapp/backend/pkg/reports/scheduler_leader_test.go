package reports

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestCheckAndRun_SkipsWhenNotLeader(t *testing.T) {
	s := NewScheduler(nil, logrus.New(), func() (database.DeviceRepo, error) {
		t.Fatal("a replica that is not leader must not touch the database")
		return nil, nil
	})
	s.SetLeaderCheck(func() bool { return false })

	s.checkAndRun()
}

// A replica that becomes leader must not resend a report that the previous leader already sent,
// even though its own copy of the last-run time is from before that run.
func TestCheckAndRun_ReadsLastRunFromDatabase(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)

	settings := &models.Settings{}
	settings.Metrics.ReportEnabled = true
	settings.Metrics.ReportDailyEnabled = true
	settings.Metrics.ReportDailyTime = "00:00"
	repo.EXPECT().LoadSettings(gomock.Any()).Return(settings, nil)
	repo.EXPECT().GetSettingValue(gomock.Any(), settingLastDailyRun).Return(time.Now().Format(time.RFC3339), nil)
	repo.EXPECT().GetSettingValue(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()
	// Any report generation would call further repository methods, which the strict mock rejects.

	s := NewScheduler(nil, logrus.New(), func() (database.DeviceRepo, error) { return repo, nil })
	s.SetLeaderCheck(func() bool { return true })

	s.checkAndRun()
}

// Two replicas that both find the same report due must not both send it.
func TestClaimRun_OnlyOneReplicaWins(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	now := time.Now()
	repo.EXPECT().GetSettingValue(gomock.Any(), settingLastDailyRun).Return("", nil).Times(2)
	gomock.InOrder(
		repo.EXPECT().CompareAndSetSettingValue(gomock.Any(), settingLastDailyRun, "", now.Format(time.RFC3339)).Return(true, nil),
		repo.EXPECT().CompareAndSetSettingValue(gomock.Any(), settingLastDailyRun, "", now.Format(time.RFC3339)).Return(false, nil),
	)
	due := func(time.Time) bool { return true }

	first := NewScheduler(nil, logrus.New(), nil)
	second := NewScheduler(nil, logrus.New(), nil)
	require.True(t, first.claimRun(repo, settingLastDailyRun, now, due))
	require.False(t, second.claimRun(repo, settingLastDailyRun, now, due))
}

func TestClaimRun_NotDueDoesNotWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().GetSettingValue(gomock.Any(), settingLastDailyRun).Return(time.Now().Format(time.RFC3339), nil)

	s := NewScheduler(nil, logrus.New(), nil)
	require.False(t, s.claimRun(repo, settingLastDailyRun, time.Now(), func(time.Time) bool { return false }))
}
