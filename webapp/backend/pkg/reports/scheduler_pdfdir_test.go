package reports

import (
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPDFDirComesFromConfigOnly(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	cfg := mock_config.NewMockInterface(mockCtrl)
	cfg.EXPECT().GetString("web.reports.path").Return("/data/reports").AnyTimes()
	s := &Scheduler{appConfig: cfg}
	require.Equal(t, "/data/reports", s.pdfDir())

	empty := mock_config.NewMockInterface(mockCtrl)
	empty.EXPECT().GetString("web.reports.path").Return("").AnyTimes()
	require.Equal(t, DefaultPDFDir, (&Scheduler{appConfig: empty}).pdfDir())
}

func TestWarnLegacyPDFPathLogsOnceWhenStoredValueDiffers(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	cfg := mock_config.NewMockInterface(mockCtrl)
	cfg.EXPECT().GetString("user.metrics.report_pdf_path").Return("/custom/path").AnyTimes()
	logger, hook := logtest.NewNullLogger()
	s := &Scheduler{appConfig: cfg, logger: logger}

	s.warnLegacyPDFPath("/opt/scrutiny/reports")
	s.warnLegacyPDFPath("/opt/scrutiny/reports")

	require.Len(t, hook.AllEntries(), 1)
	require.Contains(t, hook.LastEntry().Message, "web.reports.path")
}

func TestWarnLegacyPDFPathSilentWhenUnchanged(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	cfg := mock_config.NewMockInterface(mockCtrl)
	cfg.EXPECT().GetString("user.metrics.report_pdf_path").Return("/opt/scrutiny/reports").AnyTimes()
	logger, hook := logtest.NewNullLogger()
	(&Scheduler{appConfig: cfg, logger: logger}).warnLegacyPDFPath("/opt/scrutiny/reports")
	require.Empty(t, hook.AllEntries())
}
