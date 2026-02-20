package reports

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/sirupsen/logrus"
)

const (
	DefaultReportCheckInterval = 1 * time.Minute

	settingLastDailyRun   = "metrics.report_last_daily_run"
	settingLastWeeklyRun  = "metrics.report_last_weekly_run"
	settingLastMonthlyRun = "metrics.report_last_monthly_run"
)

// Scheduler runs report generation on configured schedules
type Scheduler struct {
	appConfig  config.Interface
	logger     logrus.FieldLogger
	deviceRepo database.DeviceRepo
	ctx        context.Context
	cancel     context.CancelFunc
	stopCh     chan struct{}
	repoFactory func() (database.DeviceRepo, error)

	lastDailyRun   time.Time
	lastWeeklyRun  time.Time
	lastMonthlyRun time.Time

	repoMu sync.Mutex
	wg     sync.WaitGroup
	mu     sync.Mutex   // guards started/stopped flags
	runMu  sync.RWMutex // guards last run timestamps

	started bool
	stopped bool
}

// NewScheduler creates a new report scheduler
func NewScheduler(appConfig config.Interface, logger logrus.FieldLogger, repoFactory func() (database.DeviceRepo, error)) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		appConfig:   appConfig,
		logger:      logger,
		stopCh:      make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
		repoFactory: repoFactory,
	}
}

// Start begins the scheduler goroutine
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.started = true
	s.wg.Add(1)
	go s.run()
}

// Stop gracefully shuts down the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	s.logger.Debug("Stopping report scheduler...")
	s.cancel()
	close(s.stopCh)
	s.wg.Wait()

	s.repoMu.Lock()
	if s.deviceRepo != nil {
		s.deviceRepo.Close()
		s.deviceRepo = nil
	}
	s.repoMu.Unlock()

	s.logger.Info("Report scheduler stopped")
}

func (s *Scheduler) run() {
	defer s.wg.Done()

	s.loadLastRunTimestamps()

	ticker := time.NewTicker(DefaultReportCheckInterval)
	defer ticker.Stop()

	s.logger.Info("Report scheduler started")

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndRun()
		}
	}
}

func (s *Scheduler) checkAndRun() {
	// Load settings fresh to check if reports are enabled
	repo, err := s.getRepo()
	if err != nil {
		s.logger.Debugf("Report scheduler: failed to get repo: %v", err)
		return
	}

	settings, err := repo.LoadSettings(s.ctx)
	if err != nil {
		s.logger.Debugf("Report scheduler: failed to load settings: %v", err)
		return
	}
	if settings == nil || !settings.Metrics.ReportEnabled {
		return
	}

	now := time.Now()

	s.runMu.RLock()
	lastDaily := s.lastDailyRun
	lastWeekly := s.lastWeeklyRun
	lastMonthly := s.lastMonthlyRun
	s.runMu.RUnlock()

	if settings.Metrics.ReportDailyEnabled {
		if isDailyDue(now, lastDaily, settings.Metrics.ReportDailyTime) {
			s.runReport("daily", now, settings.Metrics.ReportPDFEnabled, settings.Metrics.ReportPDFPath)
			s.runMu.Lock()
			s.lastDailyRun = now
			s.runMu.Unlock()
			s.saveLastRunTimestamp(settingLastDailyRun, now)
		}
	}

	if settings.Metrics.ReportWeeklyEnabled {
		if isWeeklyDue(now, lastWeekly, settings.Metrics.ReportWeeklyDay, settings.Metrics.ReportWeeklyTime) {
			s.runReport("weekly", now, settings.Metrics.ReportPDFEnabled, settings.Metrics.ReportPDFPath)
			s.runMu.Lock()
			s.lastWeeklyRun = now
			s.runMu.Unlock()
			s.saveLastRunTimestamp(settingLastWeeklyRun, now)
		}
	}

	if settings.Metrics.ReportMonthlyEnabled {
		if isMonthlyDue(now, lastMonthly, settings.Metrics.ReportMonthlyDay, settings.Metrics.ReportMonthlyTime) {
			s.runReport("monthly", now, settings.Metrics.ReportPDFEnabled, settings.Metrics.ReportPDFPath)
			s.runMu.Lock()
			s.lastMonthlyRun = now
			s.runMu.Unlock()
			s.saveLastRunTimestamp(settingLastMonthlyRun, now)
		}
	}
}

func (s *Scheduler) runReport(periodType string, now time.Time, pdfEnabled bool, pdfPath string) {
	s.logger.Infof("Generating %s report...", periodType)

	repo, err := s.getRepo()
	if err != nil {
		s.logger.Errorf("Failed to get device repo for report: %v", err)
		return
	}

	var start time.Time
	switch periodType {
	case "daily":
		start = now.Add(-24 * time.Hour)
	case "weekly":
		start = now.Add(-7 * 24 * time.Hour)
	case "monthly":
		start = now.AddDate(0, -1, 0)
	}

	gen := NewGenerator(repo)
	report, err := gen.Generate(s.ctx, periodType, start, now)
	if err != nil {
		s.logger.Errorf("Failed to generate %s report: %v", periodType, err)
		return
	}

	subject, message := FormatTextReport(report)
	htmlMessage := FormatHTMLReport(report)
	s.sendNotification(subject, message, htmlMessage)

	if pdfEnabled {
		outputDir := pdfPath
		if outputDir == "" {
			outputDir = "/opt/scrutiny/reports"
		}
		filename := PDFFilename(periodType, now)
		outputPath := filepath.Join(outputDir, filename)

		if err := GeneratePDF(report, outputPath, version.VERSION); err != nil {
			s.logger.Errorf("Failed to generate PDF report: %v", err)
		} else {
			s.logger.Infof("PDF report saved to %s", outputPath)
		}
	}

	s.logger.Infof("Completed %s report generation", periodType)
}

func (s *Scheduler) sendNotification(subject, message, htmlMessage string) {
	reportNotify := notify.NewReport(s.logger, s.appConfig, subject, message, htmlMessage)
	if err := reportNotify.Send(); err != nil {
		s.logger.Errorf("Failed to send report notification: %v", err)
	}
}

func (s *Scheduler) loadLastRunTimestamps() {
	repo, err := s.getRepo()
	if err != nil {
		s.logger.Warnf("Report scheduler: could not load last-run timestamps: %v", err)
		return
	}

	load := func(key string) time.Time {
		val, err := repo.GetSettingValue(s.ctx, key)
		if err != nil || val == "" {
			return time.Time{}
		}
		t, err := time.Parse(time.RFC3339, val)
		if err != nil {
			s.logger.Warnf("Report scheduler: invalid timestamp for %s: %q", key, val)
			return time.Time{}
		}
		return t
	}

	s.runMu.Lock()
	s.lastDailyRun = load(settingLastDailyRun)
	s.lastWeeklyRun = load(settingLastWeeklyRun)
	s.lastMonthlyRun = load(settingLastMonthlyRun)
	s.runMu.Unlock()

	s.logger.Infof("Report scheduler loaded last-run timestamps: daily=%v, weekly=%v, monthly=%v",
		s.lastDailyRun, s.lastWeeklyRun, s.lastMonthlyRun)
}

func (s *Scheduler) saveLastRunTimestamp(key string, t time.Time) {
	repo, err := s.getRepo()
	if err != nil {
		s.logger.Errorf("Report scheduler: could not save last-run timestamp: %v", err)
		return
	}
	if err := repo.SetSettingValue(s.ctx, key, t.Format(time.RFC3339)); err != nil {
		s.logger.Errorf("Report scheduler: failed to persist %s: %v", key, err)
	}
}

func (s *Scheduler) getRepo() (database.DeviceRepo, error) {
	s.repoMu.Lock()
	defer s.repoMu.Unlock()

	if s.deviceRepo != nil {
		return s.deviceRepo, nil
	}

	repo, err := s.repoFactory()
	if err != nil {
		return nil, err
	}
	s.deviceRepo = repo
	return repo, nil
}

// GenerateOnDemand generates a report immediately and returns the data.
func (s *Scheduler) GenerateOnDemand(ctx context.Context, periodType string) (*ReportData, error) {
	repo, err := s.getRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to get device repo: %w", err)
	}

	now := time.Now()
	var start time.Time
	switch periodType {
	case "daily":
		start = now.Add(-24 * time.Hour)
	case "weekly":
		start = now.Add(-7 * 24 * time.Hour)
	case "monthly":
		start = now.AddDate(0, -1, 0)
	default:
		return nil, fmt.Errorf("invalid period type: %s", periodType)
	}

	gen := NewGenerator(repo)
	return gen.Generate(ctx, periodType, start, now)
}

// SendTestReport generates a report and sends it via the notification system.
func (s *Scheduler) SendTestReport(ctx context.Context, periodType string) (*ReportData, error) {
	report, err := s.GenerateOnDemand(ctx, periodType)
	if err != nil {
		return nil, err
	}

	subject, message := FormatTextReport(report)
	htmlMessage := FormatHTMLReport(report)
	s.sendNotification(subject, message, htmlMessage)
	return report, nil
}

// GenerateOnDemandPDF generates a PDF and returns the file path.
func (s *Scheduler) GenerateOnDemandPDF(ctx context.Context, periodType string) (string, error) {
	report, err := s.GenerateOnDemand(ctx, periodType)
	if err != nil {
		return "", err
	}

	tmpDir := os.TempDir()
	filename := PDFFilename(periodType, time.Now())
	outputPath := filepath.Join(tmpDir, "scrutiny-reports", filename)

	if err := GeneratePDF(report, outputPath, version.VERSION); err != nil {
		return "", fmt.Errorf("failed to generate PDF: %w", err)
	}

	return outputPath, nil
}

// Schedule checking functions

func isDailyDue(now, lastRun time.Time, timeStr string) bool {
	h, m := parseTimeOfDay(timeStr)
	scheduledToday := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if now.Before(scheduledToday) {
		return false
	}
	if !lastRun.IsZero() && lastRun.Year() == now.Year() && lastRun.YearDay() == now.YearDay() {
		return false
	}
	return true
}

func isWeeklyDue(now, lastRun time.Time, dayOfWeek int, timeStr string) bool {
	if int(now.Weekday()) != dayOfWeek {
		return false
	}
	return isDailyDue(now, lastRun, timeStr)
}

func isMonthlyDue(now, lastRun time.Time, dayOfMonth int, timeStr string) bool {
	if now.Day() != dayOfMonth {
		return false
	}
	return isDailyDue(now, lastRun, timeStr)
}

func parseTimeOfDay(timeStr string) (int, int) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 8, 0
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 8, 0
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 8, 0
	}
	return h, m
}
