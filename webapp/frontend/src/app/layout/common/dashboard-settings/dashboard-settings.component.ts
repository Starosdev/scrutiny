import {Component, OnInit} from '@angular/core';
import {HttpClient} from '@angular/common/http';
import {
    AppConfig,
    AttributeOverride,
    DashboardDisplay,
    DashboardSort,
    MetricsNotifyLevel,
    MetricsStatusFilterAttributes,
    MetricsStatusThreshold,
    NotifyUrlEntry,
    OverrideAction,
    OverrideProtocol,
    OverrideStatus,
    TemperatureUnit,
    LineStroke,
    Theme,
    DevicePoweredOnUnit
} from 'app/core/config/app.config';
import {ScrutinyConfigService} from 'app/core/config/scrutiny-config.service';
import {AttributeOverrideService} from 'app/core/config/attribute-override.service';
import {NotifyUrlService} from 'app/core/config/notify-url.service';
import {getBasePath} from 'app/app.routing';
import {Subject} from 'rxjs';
import {takeUntil} from 'rxjs/operators';

@Component({
    selector: 'app-dashboard-settings',
    templateUrl: './dashboard-settings.component.html',
    styleUrls: ['./dashboard-settings.component.scss'],
    standalone: false
})
export class DashboardSettingsComponent implements OnInit {

    dashboardDisplay: string;
    dashboardSort: string;
    temperatureUnit: string;
    fileSizeSIUnits: boolean;
    poweredOnHoursUnit: string;
    lineStroke: string;
    theme: string;
    retrieveSCTTemperatureHistory: boolean;
    notifyLevel: number;
    statusThreshold: number;
    statusFilterAttributes: number;
    repeatNotifications: boolean;

    // Collector error settings
    notifyOnCollectorError: boolean;

    // Missed ping settings
    notifyOnMissedPing: boolean;
    missedPingTimeoutMinutes: number;
    missedPingCheckIntervalMins: number;

    // Notification cooldown / rate limiting
    missedPingCooldownMinutes: number;
    notificationRateLimit: number;

    // Quiet hours
    notificationQuietStart: string;
    notificationQuietEnd: string;

    // Heartbeat settings
    heartbeatEnabled: boolean;
    heartbeatIntervalHours: number;

    // Uptime Kuma settings
    uptimeKumaEnabled: boolean;
    uptimeKumaPushURL: string;
    uptimeKumaIntervalSeconds: number;
    uptimeKumaTestLoading = false;
    uptimeKumaTestResult: string | null = null;

    // Report settings
    reportEnabled: boolean;
    reportDailyEnabled: boolean;
    reportDailyTime: string;
    reportWeeklyEnabled: boolean;
    reportWeeklyDay: number;
    reportWeeklyTime: string;
    reportMonthlyEnabled: boolean;
    reportMonthlyDay: number;
    reportMonthlyTime: string;
    reportPDFEnabled: boolean;
    reportPDFPath: string;

    // Attribute overrides
    overrides: AttributeOverride[] = [];
    displayedColumns: string[] = ['protocol', 'attribute_id', 'action', 'source', 'actions'];
    protocols: OverrideProtocol[] = ['ATA', 'NVMe', 'SCSI'];
    actions: {value: OverrideAction, label: string}[] = [
        {value: 'ignore', label: 'Ignore'},
        {value: 'force_status', label: 'Force Status'},
        {value: '', label: 'Custom Threshold'}
    ];
    statuses: OverrideStatus[] = ['passed', 'warn', 'failed'];

    // New override form
    newOverride: Partial<AttributeOverride> = {
        protocol: 'ATA',
        attribute_id: '',
        action: 'ignore'
    };

    // Notification URL management
    notifyUrls: NotifyUrlEntry[] = [];
    notifyUrlColumns: string[] = ['label', 'url', 'source', 'actions'];
    showAddUrlForm = false;
    selectedService: 'custom' | 'smtp' | 'discord' | 'slack' | 'telegram' = 'custom';
    newUrlRaw = '';
    newUrlLabel = '';
    // SMTP fields
    smtpHost = '';
    smtpPort = '587';
    smtpUsername = '';
    smtpPassword = '';
    smtpFrom = '';
    smtpTo = '';
    // Discord
    discordWebhookUrl = '';
    // Slack
    slackWebhookUrl = '';
    // Telegram
    telegramToken = '';
    telegramChatId = '';
    // Test notification state
    testingUrlId: number | null = null;

    // Private
    private _unsubscribeAll: Subject<void>;

    constructor(
        private _configService: ScrutinyConfigService,
        private _overrideService: AttributeOverrideService,
        private _notifyUrlService: NotifyUrlService,
        private _httpClient: HttpClient,
    ) {
        // Set the private defaults
        this._unsubscribeAll = new Subject();
    }

    ngOnInit(): void {
        // Subscribe to config changes
        this._configService.config$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((config: AppConfig) => {

                // Store the config
                this.dashboardDisplay = config.dashboard_display;
                this.dashboardSort = config.dashboard_sort;
                this.temperatureUnit = config.temperature_unit;
                this.fileSizeSIUnits = config.file_size_si_units;
                this.poweredOnHoursUnit = config.powered_on_hours_unit;
                this.lineStroke = config.line_stroke;
                this.theme = config.theme;

                this.retrieveSCTTemperatureHistory = config.collector.retrieve_sct_temperature_history;

                this.notifyLevel = config.metrics.notify_level;
                this.statusFilterAttributes = config.metrics.status_filter_attributes;
                this.statusThreshold = config.metrics.status_threshold;
                this.repeatNotifications = config.metrics.repeat_notifications;

                // Collector error settings
                this.notifyOnCollectorError = config.metrics.notify_on_collector_error ?? true;

                // Missed ping settings
                this.notifyOnMissedPing = config.metrics.notify_on_missed_ping ?? false;
                this.missedPingTimeoutMinutes = config.metrics.missed_ping_timeout_minutes ?? 60;
                this.missedPingCheckIntervalMins = config.metrics.missed_ping_check_interval_mins ?? 5;

                // Notification cooldown / rate limiting
                this.missedPingCooldownMinutes = config.metrics.missed_ping_cooldown_minutes ?? 0;
                this.notificationRateLimit = config.metrics.notification_rate_limit ?? 0;

                // Quiet hours
                this.notificationQuietStart = config.metrics.notification_quiet_start ?? '';
                this.notificationQuietEnd = config.metrics.notification_quiet_end ?? '';

                // Heartbeat settings
                this.heartbeatEnabled = config.metrics.heartbeat_enabled ?? false;
                this.heartbeatIntervalHours = config.metrics.heartbeat_interval_hours ?? 24;

                // Uptime Kuma settings
                this.uptimeKumaEnabled = config.metrics.uptime_kuma_enabled ?? false;
                this.uptimeKumaPushURL = config.metrics.uptime_kuma_push_url ?? '';
                this.uptimeKumaIntervalSeconds = config.metrics.uptime_kuma_interval_seconds ?? 60;

                // Report settings
                this.reportEnabled = config.metrics.report_enabled ?? false;
                this.reportDailyEnabled = config.metrics.report_daily_enabled ?? false;
                this.reportDailyTime = config.metrics.report_daily_time ?? '08:00';
                this.reportWeeklyEnabled = config.metrics.report_weekly_enabled ?? false;
                this.reportWeeklyDay = config.metrics.report_weekly_day ?? 1;
                this.reportWeeklyTime = config.metrics.report_weekly_time ?? '08:00';
                this.reportMonthlyEnabled = config.metrics.report_monthly_enabled ?? false;
                this.reportMonthlyDay = config.metrics.report_monthly_day ?? 1;
                this.reportMonthlyTime = config.metrics.report_monthly_time ?? '08:00';
                this.reportPDFEnabled = config.metrics.report_pdf_enabled ?? false;
                this.reportPDFPath = config.metrics.report_pdf_path ?? '/opt/scrutiny/reports';

            });

        // Load attribute overrides
        this.loadOverrides();

        // Load notification URLs
        this.loadNotifyUrls();
    }

    loadOverrides(): void {
        this._overrideService.getOverrides()
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(overrides => {
                this.overrides = overrides;
            });
    }

    addOverride(): void {
        if (!this.newOverride.protocol || !this.newOverride.attribute_id) {
            return;
        }

        const override: AttributeOverride = {
            protocol: this.newOverride.protocol as OverrideProtocol,
            attribute_id: this.newOverride.attribute_id,
            action: this.newOverride.action as OverrideAction,
            wwn: this.newOverride.wwn || '',
            status: this.newOverride.status as OverrideStatus,
            warn_above: this.newOverride.warn_above,
            fail_above: this.newOverride.fail_above
        };

        this._overrideService.saveOverride(override)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(saved => {
                this.overrides = [...this.overrides, saved];
                // Reset form
                this.newOverride = {
                    protocol: 'ATA',
                    attribute_id: '',
                    action: 'ignore'
                };
            });
    }

    removeOverride(override: AttributeOverride): void {
        if (!override.id || override.source === 'config') {
            return;
        }

        this._overrideService.deleteOverride(override.id)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(() => {
                this.overrides = this.overrides.filter(o => o.id !== override.id);
            });
    }

    getActionLabel(action: string): string {
        const found = this.actions.find(a => a.value === action);
        return found ? found.label : 'Custom Threshold';
    }

    // Notification URL methods

    loadNotifyUrls(): void {
        this._notifyUrlService.getNotifyUrls()
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(urls => {
                this.notifyUrls = urls;
            });
    }

    deleteNotifyUrl(entry: NotifyUrlEntry): void {
        if (!entry.id || entry.source !== 'ui') {
            return;
        }
        this._notifyUrlService.deleteNotifyUrl(entry.id)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(() => {
                this.notifyUrls = this.notifyUrls.filter(u => u.id !== entry.id);
            });
    }

    testNotifyUrl(entry: NotifyUrlEntry): void {
        if (!entry.id) {
            return;
        }
        this.testingUrlId = entry.id;
        this._notifyUrlService.testNotifyUrl(entry.id)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe({
                next: () => { this.testingUrlId = null; },
                error: () => { this.testingUrlId = null; }
            });
    }

    buildShoutrrrUrl(): string {
        switch (this.selectedService) {
            case 'custom':
                return this.newUrlRaw.trim();
            case 'smtp': {
                if (!this.smtpHost || !this.smtpFrom || !this.smtpTo) {
                    return '';
                }
                const user = encodeURIComponent(this.smtpUsername);
                const pass = encodeURIComponent(this.smtpPassword);
                const auth = this.smtpUsername ? `${user}:${pass}@` : '';
                return `smtp://${auth}${this.smtpHost}:${this.smtpPort}/?from=${encodeURIComponent(this.smtpFrom)}&to=${encodeURIComponent(this.smtpTo)}`;
            }
            case 'discord': {
                const match = this.discordWebhookUrl.match(/webhooks\/(\d+)\/([^\/\?]+)/);
                return match ? `discord://${match[2]}@${match[1]}` : '';
            }
            case 'slack': {
                const match = this.slackWebhookUrl.match(/services\/([^\/]+)\/([^\/]+)\/([^\/\?]+)/);
                return match ? `slack://hook:${match[1]}/${match[2]}/${match[3]}` : '';
            }
            case 'telegram':
                return (this.telegramToken && this.telegramChatId)
                    ? `telegram://${this.telegramToken}@telegram?chats=${this.telegramChatId}`
                    : '';
            default:
                return '';
        }
    }

    addNotifyUrl(): void {
        const url = this.buildShoutrrrUrl();
        if (!url) {
            return;
        }
        const label = this.newUrlLabel.trim();

        this._notifyUrlService.addNotifyUrl(url, label)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(saved => {
                this.notifyUrls = [...this.notifyUrls, saved];
                this.showAddUrlForm = false;
                this.resetAddForm();
            });
    }

    resetAddForm(): void {
        this.selectedService = 'custom';
        this.newUrlRaw = '';
        this.newUrlLabel = '';
        this.smtpHost = '';
        this.smtpPort = '587';
        this.smtpUsername = '';
        this.smtpPassword = '';
        this.smtpFrom = '';
        this.smtpTo = '';
        this.discordWebhookUrl = '';
        this.slackWebhookUrl = '';
        this.telegramToken = '';
        this.telegramChatId = '';
    }

    saveSettings(): void {
        const newSettings: AppConfig = {
            dashboard_display: this.dashboardDisplay as DashboardDisplay,
            dashboard_sort: this.dashboardSort as DashboardSort,
            temperature_unit: this.temperatureUnit as TemperatureUnit,
            file_size_si_units: this.fileSizeSIUnits,
            powered_on_hours_unit: this.poweredOnHoursUnit as DevicePoweredOnUnit,
            line_stroke: this.lineStroke as LineStroke,
            theme: this.theme as Theme,
            collector: {
                retrieve_sct_temperature_history: this.retrieveSCTTemperatureHistory
            },
            metrics: {
                notify_level: this.notifyLevel as MetricsNotifyLevel,
                status_filter_attributes: this.statusFilterAttributes as MetricsStatusFilterAttributes,
                status_threshold: this.statusThreshold as MetricsStatusThreshold,
                repeat_notifications: this.repeatNotifications,
                notify_on_collector_error: this.notifyOnCollectorError,
                notify_on_missed_ping: this.notifyOnMissedPing,
                missed_ping_timeout_minutes: this.missedPingTimeoutMinutes,
                missed_ping_check_interval_mins: this.missedPingCheckIntervalMins,
                missed_ping_cooldown_minutes: this.missedPingCooldownMinutes,
                notification_rate_limit: this.notificationRateLimit,
                notification_quiet_start: this.notificationQuietStart,
                notification_quiet_end: this.notificationQuietEnd,
                heartbeat_enabled: this.heartbeatEnabled,
                heartbeat_interval_hours: this.heartbeatIntervalHours,
                uptime_kuma_enabled: this.uptimeKumaEnabled,
                uptime_kuma_push_url: this.uptimeKumaPushURL,
                uptime_kuma_interval_seconds: this.uptimeKumaIntervalSeconds,
                report_enabled: this.reportEnabled,
                report_daily_enabled: this.reportDailyEnabled,
                report_daily_time: this.reportDailyTime,
                report_weekly_enabled: this.reportWeeklyEnabled,
                report_weekly_day: this.reportWeeklyDay,
                report_weekly_time: this.reportWeeklyTime,
                report_monthly_enabled: this.reportMonthlyEnabled,
                report_monthly_day: this.reportMonthlyDay,
                report_monthly_time: this.reportMonthlyTime,
                report_pdf_enabled: this.reportPDFEnabled,
                report_pdf_path: this.reportPDFPath
            }
        }
        this._configService.config = newSettings
    }

    testUptimeKuma(): void {
        this.uptimeKumaTestLoading = true;
        this.uptimeKumaTestResult = null;
        this._httpClient.post<{success: boolean; errors?: string[]}>(
            getBasePath() + '/api/health/uptime-kuma-test', {}
        ).pipe(takeUntil(this._unsubscribeAll))
            .subscribe({
                next: (resp) => {
                    this.uptimeKumaTestLoading = false;
                    this.uptimeKumaTestResult = resp.success ? 'success' : 'error';
                },
                error: () => {
                    this.uptimeKumaTestLoading = false;
                    this.uptimeKumaTestResult = 'error';
                }
            });
    }

    formatLabel(value: number): number {
        return value;
    }
}
