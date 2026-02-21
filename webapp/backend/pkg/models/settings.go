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

		// Heartbeat notification settings
		HeartbeatEnabled       bool `json:"heartbeat_enabled" mapstructure:"heartbeat_enabled"`
		HeartbeatIntervalHours int  `json:"heartbeat_interval_hours" mapstructure:"heartbeat_interval_hours"`

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
