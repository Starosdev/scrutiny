import {Component, OnInit} from '@angular/core';
import {
    AppConfig,
    AttributeOverride,
    DashboardDisplay,
    DashboardSort,
    MetricsStatusFilterAttributes,
    MetricsStatusThreshold,
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
    statusThreshold: number;
    statusFilterAttributes: number;
    repeatNotifications: boolean;

    // Missed ping settings
    notifyOnMissedPing: boolean;
    missedPingTimeoutMinutes: number;
    missedPingCheckIntervalMins: number;

    // Heartbeat settings
    heartbeatEnabled: boolean;
    heartbeatIntervalHours: number;

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

    // Private
    private _unsubscribeAll: Subject<void>;

    constructor(
        private _configService: ScrutinyConfigService,
        private _overrideService: AttributeOverrideService,
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

                this.statusFilterAttributes = config.metrics.status_filter_attributes;
                this.statusThreshold = config.metrics.status_threshold;
                this.repeatNotifications = config.metrics.repeat_notifications;

                // Missed ping settings
                this.notifyOnMissedPing = config.metrics.notify_on_missed_ping ?? false;
                this.missedPingTimeoutMinutes = config.metrics.missed_ping_timeout_minutes ?? 60;
                this.missedPingCheckIntervalMins = config.metrics.missed_ping_check_interval_mins ?? 5;

                // Heartbeat settings
                this.heartbeatEnabled = config.metrics.heartbeat_enabled ?? false;
                this.heartbeatIntervalHours = config.metrics.heartbeat_interval_hours ?? 24;

            });

        // Load attribute overrides
        this.loadOverrides();
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
                status_filter_attributes: this.statusFilterAttributes as MetricsStatusFilterAttributes,
                status_threshold: this.statusThreshold as MetricsStatusThreshold,
                repeat_notifications: this.repeatNotifications,
                notify_on_missed_ping: this.notifyOnMissedPing,
                missed_ping_timeout_minutes: this.missedPingTimeoutMinutes,
                missed_ping_check_interval_mins: this.missedPingCheckIntervalMins,
                heartbeat_enabled: this.heartbeatEnabled,
                heartbeat_interval_hours: this.heartbeatIntervalHours
            }
        }
        this._configService.config = newSettings
    }

    formatLabel(value: number): number {
        return value;
    }
}
