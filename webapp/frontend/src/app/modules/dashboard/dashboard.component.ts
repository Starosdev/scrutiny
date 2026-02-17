import {
    AfterViewInit,
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    OnDestroy,
    OnInit,
    ViewChild,
    ViewEncapsulation
} from '@angular/core';
import {Subject} from 'rxjs';
import {takeUntil} from 'rxjs/operators';
import {ApexOptions, ChartComponent} from 'ng-apexcharts';
import {DashboardService} from 'app/modules/dashboard/dashboard.service';
import {MatDialog as MatDialog} from '@angular/material/dialog';
import {DashboardSettingsComponent} from 'app/layout/common/dashboard-settings/dashboard-settings.component';
import {AppConfig} from 'app/core/config/app.config';
import {ScrutinyConfigService} from 'app/core/config/scrutiny-config.service';
import {Router} from '@angular/router';
import {TemperaturePipe} from 'app/shared/temperature.pipe';
import {DeviceTitlePipe} from 'app/shared/device-title.pipe';
import {DeviceSummaryModel} from 'app/core/models/device-summary-model';

@Component({
    selector: 'example',
    templateUrl: './dashboard.component.html',
    styleUrls: ['./dashboard.component.scss'],
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class DashboardComponent implements OnInit, AfterViewInit, OnDestroy
{
    summaryData: { [key: string]: DeviceSummaryModel };
    hostGroups: { [hostId: string]: string[] } = {}
    temperatureOptions: ApexOptions;
    tempDurationKey = 'forever'
    config: AppConfig;
    showArchived: boolean;
    visibleDrives: { [wwn: string]: boolean } = {};

    // Private
    private _unsubscribeAll: Subject<void>;
    private readonly systemPrefersDark: boolean;
    @ViewChild('tempChart', { static: false }) tempChart: ChartComponent;

    /**
     * Constructor
     *
     * @param {DashboardService} _dashboardService
     * @param {ScrutinyConfigService} _configService
     * @param {MatDialog} dialog
     * @param {Router} router
     */
    constructor(
        private _dashboardService: DashboardService,
        private _configService: ScrutinyConfigService,
        private _changeDetectorRef: ChangeDetectorRef,
        public dialog: MatDialog,
        private router: Router,
    )
    {
        // Set the private defaults
        this._unsubscribeAll = new Subject();
        this.systemPrefersDark = globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false;
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Lifecycle hooks
    // -----------------------------------------------------------------------------------------------------

    /**
     * On init
     */
    ngOnInit(): void
    {

        // Subscribe to config changes
        this._configService.config$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((config: AppConfig) => {

                // check if the old config and the new config do not match.
                const oldConfig = JSON.stringify(this.config)
                const newConfig = JSON.stringify(config)

                if(oldConfig !== newConfig){
                    // Store the config
                    this.config = config;

                    if(oldConfig){
                        this.refreshComponent()
                    }
                }
            });

        // Get the data
        this._dashboardService.data$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((data) => {

                // Store the data
                this.summaryData = data;

                // generate group data.
                for (const wwn in this.summaryData) {
                    const hostid = this.summaryData[wwn].device.host_id
                    const hostDeviceList = this.hostGroups[hostid] || []
                    hostDeviceList.push(wwn)
                    this.hostGroups[hostid] = hostDeviceList

                    // Initialize drive visibility (default to visible)
                    this.visibleDrives[wwn] ??= true;
                }
                // Prepare the chart data
                this._prepareChartData();
            });
    }

    /**
     * After view init
     */
    ngAfterViewInit(): void
    {}

    /**
     * On destroy
     */
    ngOnDestroy(): void
    {
        // Unsubscribe from all subscriptions
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Private methods
    // -----------------------------------------------------------------------------------------------------
    private refreshComponent(): void {

        const currentUrl = this.router.url;
        this.router.routeReuseStrategy.shouldReuseRoute = () => false;
        this.router.onSameUrlNavigation = 'reload';
        this.router.navigate([currentUrl]);
    }

    private _deviceDataTemperatureSeries(): any[] {
        const deviceTemperatureSeries = []

        for (const wwn in this.summaryData) {
            // Skip drives that are hidden by the filter
            if (this.visibleDrives[wwn] === false) {
                continue
            }

            const deviceSummary = this.summaryData[wwn]
            if (!deviceSummary.temp_history) {
                continue
            }

            const deviceName = DeviceTitlePipe.deviceTitleWithFallback(deviceSummary.device, this.config.dashboard_display)

            const deviceSeriesMetadata = {
                name: deviceName,
                data: []
            }

            for(const tempHistory of deviceSummary.temp_history){
                const newDate = new Date(tempHistory.date);
                let temperature;
                switch (this.config.temperature_unit) {
                    case 'celsius':
                        temperature = tempHistory.temp;
                        break
                    case 'fahrenheit':
                        temperature = TemperaturePipe.celsiusToFahrenheit(tempHistory.temp)
                        break
                }
                deviceSeriesMetadata.data.push({
                    x: newDate,
                    y: temperature
                })
            }
            deviceTemperatureSeries.push(deviceSeriesMetadata)
        }
        return deviceTemperatureSeries
    }

    private _patchSharedTooltip(chartContext: any): void {
        try {
            const tooltip = chartContext.w.globals.tooltip;
            if (tooltip?.tooltipUtil) {
                tooltip.tooltipUtil.isInitialSeriesSameLen = () => true;
                tooltip.tooltipUtil.isXoverlap = () => true;
            }
        } catch (e) {
            // Silently fail if ApexCharts internals change
        }
    }

    private determineTheme(config: AppConfig): string {
        if (config?.theme === 'system') {
            return this.systemPrefersDark ? 'dark' : 'light';
        }
        return config?.theme || 'light';
    }

    private isDarkMode(): boolean {
        return this.determineTheme(this.config) === 'dark';
    }

    /**
     * Prepare the chart data from the data
     *
     * @private
     */
    private _prepareChartData(): void
    {
        const temperatureUnit = this.config.temperature_unit === 'celsius' ? 'C' : 'F';

        this.temperatureOptions = {
            chart  : {
                animations: {
                    speed           : 400,
                    animateGradually: {
                        enabled: false
                    }
                },
                fontFamily: 'inherit',
                foreColor : 'inherit',
                width     : '100%',
                height    : '100%',
                type      : 'area',
                sparkline : {
                    enabled: false
                },
                toolbar: {
                    show: false
                },
                events: {
                    mounted: (chartContext) => {
                        this._patchSharedTooltip(chartContext);
                    },
                    updated: (chartContext) => {
                        this._patchSharedTooltip(chartContext);
                    }
                }
            },
            colors : ['#667eea', '#9066ea', '#66c0ea', '#66ead2', '#d266ea', '#66ea90'],
            fill   : {
                colors : ['#b2bef4', '#c7b2f4', '#b2dff4', '#b2f4e8', '#e8b2f4', '#b2f4c7'],
                opacity: 0.5,
                type   : 'gradient'
            },
            series : this._deviceDataTemperatureSeries(),
            stroke : {
                curve: this.config.line_stroke,
                width: 2
            },
            markers: {
                size: 0,
                hover: {
                    sizeOffset: 4
                }
            },
            dataLabels: {
                enabled: false
            },
            tooltip: {
                theme: 'dark',
                shared: true,
                intersect: false,
                x: {
                    format: 'MMM dd, yyyy HH:mm:ss'
                },
                y: {
                    formatter: (value) => {
                        if (value === null || value === undefined) { return null; }
                        return TemperaturePipe.formatTemperature(value, this.config.temperature_unit, true) as string;
                    }
                }
            },
            xaxis: {
                type: 'datetime',
                tooltip: {
                    enabled: false
                },
                labels: {
                    datetimeUTC: false,
                    style: {
                        fontSize: '11px',
                        colors: this.isDarkMode() ? '#9ca3af' : '#6b7280'
                    },
                    datetimeFormatter: {
                        year: 'yyyy',
                        month: "MMM 'yy",
                        day: 'dd MMM',
                        hour: 'HH:mm'
                    }
                }
            },
            yaxis: {
                labels: {
                    formatter: (value) => {
                        return `${Math.round(value)}${temperatureUnit}`;
                    },
                    style: {
                        fontSize: '11px',
                        colors: this.isDarkMode() ? '#9ca3af' : '#6b7280'
                    }
                },
                title: {
                    text: `Temperature (${temperatureUnit})`,
                    style: {
                        fontSize: '12px',
                        color: this.isDarkMode() ? '#9ca3af' : '#6b7280'
                    }
                }
            },
            grid: {
                borderColor: this.isDarkMode() ? '#374151' : '#e0e0e0',
                strokeDashArray: 4,
                yaxis: {
                    lines: {
                        show: true
                    }
                },
                xaxis: {
                    lines: {
                        show: false
                    }
                },
                padding: {
                    left: 10,
                    right: 10
                }
            }
        };
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    deviceSummariesForHostGroup(hostGroupWWNs: string[]): DeviceSummaryModel[] {
        const deviceSummaries: DeviceSummaryModel[] = []
        for (const wwn of hostGroupWWNs) {
            if (this.summaryData[wwn]) {
                deviceSummaries.push(this.summaryData[wwn])
            }
        }
        return deviceSummaries
    }

    /**
     * Get the collector version for a host group (from first device in group)
     */
    getCollectorVersionForHost(hostGroupWWNs: string[]): string | null {
        for (const wwn of hostGroupWWNs) {
            const version = this.summaryData[wwn]?.device?.collector_version;
            if (version) {
                return version;
            }
        }
        return null;
    }

    /**
     * Check if host's collector version is older than server version
     */
    isHostCollectorOutdated(hostGroupWWNs: string[]): boolean {
        const collectorVersion = this.getCollectorVersionForHost(hostGroupWWNs);
        const serverVersion = this.config?.server_version;

        if (!collectorVersion || !serverVersion) {
            return false;
        }

        return collectorVersion < serverVersion;
    }

    openDialog(): void {
        const dialogRef = this.dialog.open(DashboardSettingsComponent, {width: '600px',});

        dialogRef.afterClosed().subscribe();
    }

    onDeviceDeleted(wwn: string): void {
        delete this.summaryData[wwn] // remove the device from the summary list.
    }

    onDeviceArchived(wwn: string): void {
        this.summaryData[wwn].device.archived = true;
    }

    onDeviceUnarchived(wwn: string): void {
        this.summaryData[wwn].device.archived = false;
    }

    get allDrivesVisible(): boolean {
        const wwns = Object.keys(this.visibleDrives);
        return wwns.length > 0 && wwns.every(wwn => this.visibleDrives[wwn]);
    }

    get someDrivesVisible(): boolean {
        const wwns = Object.keys(this.visibleDrives);
        return wwns.some(wwn => this.visibleDrives[wwn]);
    }

    toggleAllDrives(): void {
        const newState = !this.allDrivesVisible;
        for (const wwn in this.visibleDrives) {
            this.visibleDrives[wwn] = newState;
        }
        this.tempChart?.updateSeries(this._deviceDataTemperatureSeries());
        this._changeDetectorRef.markForCheck();
    }

    toggleDriveVisibility(wwn: string): void {
        this.visibleDrives[wwn] = !this.visibleDrives[wwn];
        this.tempChart?.updateSeries(this._deviceDataTemperatureSeries());
        this._changeDetectorRef.markForCheck();
    }

    /*
    DURATION_KEY_DAY    = "day"
    DURATION_KEY_WEEK    = "week"
    DURATION_KEY_MONTH   = "month"
    DURATION_KEY_YEAR    = "year"
    DURATION_KEY_FOREVER = "forever"
     */

    changeSummaryTempDuration(durationKey: string): void {
        this.tempDurationKey = durationKey

        this._dashboardService.getSummaryTempData(durationKey)
            .subscribe((tempHistoryData) => {

                // given a list of device temp history, override the data in the "summary" object.
                for (const wwn in this.summaryData) {
                    this.summaryData[wwn].temp_history = tempHistoryData[wwn] || []
                }

                // Prepare the chart series data (filtered by visibility)
                this.tempChart.updateSeries(this._deviceDataTemperatureSeries());
            });
    }

    /**
     * Track by function for ngFor loops
     *
     * @param index
     * @param item
     */
    trackByFn(index: number, item: any): any
    {
        return item.id || index;
    }

}
