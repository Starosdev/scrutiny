package models

// Settings is made up of parsed SettingEntry objects retrieved from the database
//type Settings struct {
//	MetricsNotifyLevel            pkg.MetricsNotifyLevel            `json:"metrics.notify.level" mapstructure:"metrics.notify.level"`
//	MetricsStatusFilterAttributes pkg.MetricsStatusFilterAttributes `json:"metrics.status.filter_attributes" mapstructure:"metrics.status.filter_attributes"`
//	MetricsStatusThreshold        pkg.MetricsStatusThreshold        `json:"metrics.status.threshold" mapstructure:"metrics.status.threshold"`
//}

type Settings struct {
	Theme              string `json:"theme" mapstructure:"theme"`
	Layout             string `json:"layout" mapstructure:"layout"`
	DashboardDisplay   string `json:"dashboard_display" mapstructure:"dashboard_display"`
	DashboardSort      string `json:"dashboard_sort" mapstructure:"dashboard_sort"`
	TemperatureUnit    string `json:"temperature_unit" mapstructure:"temperature_unit"`
	FileSizeSIUnits    bool   `json:"file_size_si_units" mapstructure:"file_size_si_units"`
	LineStroke         string `json:"line_stroke" mapstructure:"line_stroke"`
	PoweredOnHoursUnit string `json:"powered_on_hours_unit" mapstructure:"powered_on_hours_unit"`

	Collector struct {
		RetrieveSCTHistory bool `json:"retrieve_sct_temperature_history" mapstructure:"retrieve_sct_temperature_history"`
	} `json:"collector" mapstructure:"collector"`

	Metrics struct {
		NotifyLevel            int  `json:"notify_level" mapstructure:"notify_level"`
		StatusFilterAttributes int  `json:"status_filter_attributes" mapstructure:"status_filter_attributes"`
		StatusThreshold        int  `json:"status_threshold" mapstructure:"status_threshold"`
		RepeatNotifications    bool `json:"repeat_notifications" mapstructure:"repeat_notifications"`

		// Missed collector ping notification settings
		NotifyOnMissedPing          bool `json:"notify_on_missed_ping" mapstructure:"notify_on_missed_ping"`
		MissedPingTimeoutMinutes    int  `json:"missed_ping_timeout_minutes" mapstructure:"missed_ping_timeout_minutes"`
		MissedPingCheckIntervalMins int  `json:"missed_ping_check_interval_mins" mapstructure:"missed_ping_check_interval_mins"`

		// Notification cooldown / rate limiting
		MissedPingCooldownMinutes int `json:"missed_ping_cooldown_minutes" mapstructure:"missed_ping_cooldown_minutes"`
		NotificationRateLimit     int `json:"notification_rate_limit" mapstructure:"notification_rate_limit"`

		// Quiet hours
		NotificationQuietStart string `json:"notification_quiet_start" mapstructure:"notification_quiet_start"`
		NotificationQuietEnd   string `json:"notification_quiet_end" mapstructure:"notification_quiet_end"`

		// Collector error notification settings
		NotifyOnCollectorError bool `json:"notify_on_collector_error" mapstructure:"notify_on_collector_error"`

		// Replacement risk notification settings
		NotifyOnReplacementRisk         bool   `json:"notify_on_replacement_risk" mapstructure:"notify_on_replacement_risk"`
		ReplacementRiskNotifyCategory   string `json:"replacement_risk_notify_category" mapstructure:"replacement_risk_notify_category"`

		// Heartbeat notification settings
		HeartbeatEnabled       bool `json:"heartbeat_enabled" mapstructure:"heartbeat_enabled"`
		HeartbeatIntervalHours int  `json:"heartbeat_interval_hours" mapstructure:"heartbeat_interval_hours"`

		// Uptime Kuma push monitor settings
		UptimeKumaEnabled         bool   `json:"uptime_kuma_enabled" mapstructure:"uptime_kuma_enabled"`
		UptimeKumaPushURL         string `json:"uptime_kuma_push_url" mapstructure:"uptime_kuma_push_url"`
		UptimeKumaIntervalSeconds int    `json:"uptime_kuma_interval_seconds" mapstructure:"uptime_kuma_interval_seconds"`

		// Scheduled report settings
		ReportEnabled        bool   `json:"report_enabled" mapstructure:"report_enabled"`
		ReportDailyEnabled   bool   `json:"report_daily_enabled" mapstructure:"report_daily_enabled"`
		ReportDailyTime      string `json:"report_daily_time" mapstructure:"report_daily_time"`
		ReportWeeklyEnabled  bool   `json:"report_weekly_enabled" mapstructure:"report_weekly_enabled"`
		ReportWeeklyDay      int    `json:"report_weekly_day" mapstructure:"report_weekly_day"`
		ReportWeeklyTime     string `json:"report_weekly_time" mapstructure:"report_weekly_time"`
		ReportMonthlyEnabled bool   `json:"report_monthly_enabled" mapstructure:"report_monthly_enabled"`
		ReportMonthlyDay     int    `json:"report_monthly_day" mapstructure:"report_monthly_day"`
		ReportMonthlyTime    string `json:"report_monthly_time" mapstructure:"report_monthly_time"`
		ReportPDFEnabled     bool   `json:"report_pdf_enabled" mapstructure:"report_pdf_enabled"`
		ReportPDFPath        string `json:"report_pdf_path" mapstructure:"report_pdf_path"`
	} `json:"metrics" mapstructure:"metrics"`
}

// defaultStr sets *p to def if *p is empty.
func defaultStr(p *string, def string) {
	if *p == "" {
		*p = def
	}
}

// defaultInt sets *p to def if *p is zero.
func defaultInt(p *int, def int) {
	if *p == 0 {
		*p = def
	}
}

// ApplyDefaults fills in zero-value fields with known-good defaults.
// This prevents the API from returning empty strings that break the frontend
// (e.g. theme="" produces invalid CSS class "treo-theme-", layout="" matches
// no template case). Called after loading settings from the database.
//
// Note: bool fields whose intended default is true (e.g. NotifyOnCollectorError,
// RepeatNotifications, NotifyOnMissedPing) cannot be safely defaulted here because
// Go's zero value for bool is false, making it impossible to distinguish between
// "user explicitly set to false" and "setting was never written to the database".
// These fields rely on the database migration to seed the correct default value on
// fresh installations. ApplyDefaults only covers string and int fields where 0/"" is
// an unambiguous sentinel for "not configured".
func (s *Settings) ApplyDefaults() {
	// Top-level string settings
	defaultStr(&s.Theme, "system")
	defaultStr(&s.Layout, "material")
	defaultStr(&s.DashboardDisplay, "name")
	defaultStr(&s.DashboardSort, "status")
	defaultStr(&s.TemperatureUnit, "celsius")
	defaultStr(&s.LineStroke, "smooth")
	defaultStr(&s.PoweredOnHoursUnit, "humanize")

	// Metrics: numeric fields where 0 is not a valid value.
	// Note: StatusFilterAttributes defaults to 0 (All), which is the zero value, so no check needed.
	defaultInt(&s.Metrics.NotifyLevel, 2)            // MetricsNotifyLevelFail
	defaultInt(&s.Metrics.StatusThreshold, 3)         // MetricsStatusThresholdBoth
	defaultInt(&s.Metrics.MissedPingTimeoutMinutes, 60)
	defaultInt(&s.Metrics.MissedPingCheckIntervalMins, 5)
	defaultInt(&s.Metrics.HeartbeatIntervalHours, 24)
	defaultInt(&s.Metrics.UptimeKumaIntervalSeconds, 60)

	// Replacement risk notification default
	defaultStr(&s.Metrics.ReplacementRiskNotifyCategory, "replace_soon")

	// Metrics: scheduled report defaults
	defaultStr(&s.Metrics.ReportDailyTime, "08:00")
	defaultInt(&s.Metrics.ReportWeeklyDay, 1)  // Monday
	defaultStr(&s.Metrics.ReportWeeklyTime, "08:00")
	defaultInt(&s.Metrics.ReportMonthlyDay, 1) // 1st of the month
	defaultStr(&s.Metrics.ReportMonthlyTime, "08:00")
	defaultStr(&s.Metrics.ReportPDFPath, "/opt/scrutiny/reports")
}
