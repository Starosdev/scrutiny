package reports

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
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
