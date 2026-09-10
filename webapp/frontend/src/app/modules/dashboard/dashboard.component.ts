import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnDestroy, OnInit, ViewEncapsulation, inject } from '@angular/core';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { ApexOptions, ChartComponent } from 'ng-apexcharts';
import { DashboardService } from 'app/modules/dashboard/dashboard.service';
import { MatDialog as MatDialog } from '@angular/material/dialog';
import { DashboardSettingsComponent } from 'app/layout/common/dashboard-settings/dashboard-settings.component';
import { AppConfig, DashboardColumns, DashboardDensity, DashboardHostPageSize } from 'app/core/config/app.config';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { Router } from '@angular/router';
import { TemperaturePipe } from 'app/shared/temperature.pipe';
import { DeviceTitlePipe } from 'app/shared/device-title.pipe';
import { DeviceSummaryModel } from 'app/core/models/device-summary-model';
import { FilesystemCapacityModel, FilesystemHostStatusModel } from 'app/core/models/filesystem-summary-model';
import { MatButton, MatIconButton } from '@angular/material/button';
import { MatIcon } from '@angular/material/icon';
import { MatTooltip } from '@angular/material/tooltip';
import { MatProgressSpinner } from '@angular/material/progress-spinner';
import { MatMenuTrigger, MatMenu, MatMenuItem } from '@angular/material/menu';
import { MenuTriggerRestoreFocusDirective } from 'app/shared/menu-trigger-restore-focus.directive';
import { DashboardDeviceComponent } from '../../layout/common/dashboard-device/dashboard-device.component';
import { DOCUMENT, NgClass, DecimalPipe, TitleCasePipe, DatePipe, KeyValuePipe } from '@angular/common';
import { MatCheckbox } from '@angular/material/checkbox';
import { MatDivider } from '@angular/material/divider';
import { FileSizePipe } from '../../shared/file-size.pipe';
import { DeviceSortPipe } from '../../shared/device-sort.pipe';
import { MatPaginator, PageEvent } from '@angular/material/paginator';
import { DeviceSummaryPagination } from 'app/core/models/device-summary-response-wrapper';
import { TemperatureDeviceOption } from 'app/core/models/device-summary-temp-response-wrapper';
import { SmartTemperatureModel } from 'app/core/models/measurements/smart-temperature-model';
import { MatInput } from '@angular/material/input';
import { MatFormField, MatLabel, MatPrefix } from '@angular/material/form-field';
import { FormsModule } from '@angular/forms';
import { alignTemperatureChartSeries, TemperatureChartSeries } from './temperature-chart-series';
import { createTemperatureChartTooltip } from './temperature-chart-tooltip';

const DASHBOARD_SHELL_WIDTHS: Record<DashboardColumns, string> = {
    2: '1440px',
    3: '1680px',
    4: '1920px',
    5: '2240px',
};

@Component({
    selector: 'example',
    templateUrl: './dashboard.component.html',
    styleUrls: ['./dashboard.component.scss'],
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [
        MatButton,
        MatIcon,
        MatTooltip,
        MatProgressSpinner,
        MatIconButton,
        MatMenuTrigger,
        MenuTriggerRestoreFocusDirective,
        MatMenu,
        MatMenuItem,
        DashboardDeviceComponent,
        NgClass,
        MatCheckbox,
        MatDivider,
        ChartComponent,
        DecimalPipe,
        TitleCasePipe,
        DatePipe,
        KeyValuePipe,
        FileSizePipe,
        DeviceSortPipe,
        MatPaginator,
        MatInput,
        MatFormField,
        MatLabel,
        MatPrefix,
        FormsModule,
    ],
})
export class DashboardComponent implements OnInit, OnDestroy {
    private readonly _dashboardService = inject(DashboardService);
    private readonly _configService = inject(ScrutinyConfigService);
    private readonly _changeDetectorRef = inject(ChangeDetectorRef);
    private readonly _document = inject(DOCUMENT);
    dialog = inject(MatDialog);
    private readonly router = inject(Router);

    summaryData: { [key: string]: DeviceSummaryModel };
    hostGroups: { [hostId: string]: string[] } = {};
    filesystemSummaryData: { filesystems: Record<string, FilesystemCapacityModel[]>; hosts: Record<string, FilesystemHostStatusModel> } | null = null;
    temperatureOptions: ApexOptions;
    tempDurationKey = 'week';
    config: AppConfig;
    showArchived: boolean = false;
    temperatureDevices: TemperatureDeviceOption[] = [];
    temperatureDeviceSearch = '';
    temperatureHistory: { [deviceID: string]: SmartTemperatureModel[] } = {};
    readonly temperatureSelection = this._dashboardService.temperatureSelection;
    hostSearch = '';
    appliedHostSearch = '';
    isTriggering: boolean = false;
    countdown: number = 0;
    pagination: DeviceSummaryPagination = {
        page: 1,
        page_size: 10,
        total_items: 0,
        total_pages: 0,
        attention_count: 0,
    };
    readonly pageSizeOptions: DashboardHostPageSize[] = [5, 10, 25, 50];

    // Private
    private readonly _unsubscribeAll: Subject<void>;
    private readonly systemPrefersDark: boolean;
    private temperatureRequestID = 0;
    /**
     * Constructor
     *
     * @param {DashboardService} _dashboardService
     * @param {ScrutinyConfigService} _configService
     * @param {MatDialog} dialog
     * @param {Router} router
     */
    constructor() {
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
    ngOnInit(): void {
        // Subscribe to config changes
        this._configService.config$.pipe(takeUntil(this._unsubscribeAll)).subscribe((config: AppConfig) => {
            // check if the old config and the new config do not match.
            const oldConfig = JSON.stringify(this.config);
            const newConfig = JSON.stringify(config);

            if (oldConfig !== newConfig) {
                // Store the config
                this.config = config;
                this._syncDashboardShellMaxWidth();

                if (oldConfig) {
                    this.refreshComponent();
                }
            }
        });

        // Get the data
        this._dashboardService.pageData$.pipe(takeUntil(this._unsubscribeAll)).subscribe((data) => {
            if (!data) {
                return;
            }

            // Store the data
            this.summaryData = data.summary;
            this.pagination = data.pagination;
            this.hostGroups = {};

            // generate group data.
            for (const wwn in this.summaryData) {
                const hostid = this.summaryData[wwn].device.host_id;
                const hostDeviceList = this.hostGroups[hostid] || [];
                hostDeviceList.push(wwn);
                this.hostGroups[hostid] = hostDeviceList;
            }
            // Prepare the chart data
            this._prepareChartData();
            this._changeDetectorRef.markForCheck();
        });
        this._dashboardService
            .getTemperatureDeviceOptions()
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((devices) => {
                this.temperatureDevices = devices;
                this.temperatureSelection.restore(devices.map((device) => device.device_id));
                if (this.temperatureSelection.ids.length > 0) {
                    this.loadSelectedTemperatureHistory();
                }
                this._changeDetectorRef.markForCheck();
            });

        this._dashboardService
            .getFilesystemSummaryData()
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((data) => {
                this.filesystemSummaryData = data;
                this._changeDetectorRef.markForCheck();
            });
    }

    /**
     * On destroy
     */
    ngOnDestroy(): void {
        this._clearDashboardShellMaxWidth();

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

    private _syncDashboardShellMaxWidth(): void {
        this._document.documentElement.style.setProperty('--scrutiny-shell-max-width', DASHBOARD_SHELL_WIDTHS[this.dashboardColumns()]);
    }

    private _clearDashboardShellMaxWidth(): void {
        this._document.documentElement.style.removeProperty('--scrutiny-shell-max-width');
    }

    deviceDashboardTitle(deviceSummary: DeviceSummaryModel): string {
        return DeviceTitlePipe.deviceDashboardTitle(deviceSummary.device);
    }

    private _deviceDataTemperatureSeries(): TemperatureChartSeries[] {
        const deviceTemperatureSeries: TemperatureChartSeries[] = [];

        for (const deviceID of this.temperatureSelection.ids) {
            const tempHistory = this.temperatureHistory[deviceID];
            if (!tempHistory) {
                continue;
            }

            const deviceName = this.temperatureDeviceTitle(this.temperatureDevices.find((device) => device.device_id === deviceID));

            const deviceSeriesMetadata: TemperatureChartSeries = {
                name: deviceName,
                data: [],
            };

            for (const measurement of tempHistory) {
                const newDate = new Date(measurement.date);
                let temperature;
                switch (this.config.temperature_unit) {
                    case 'celsius':
                        temperature = measurement.temp;
                        break;
                    case 'fahrenheit':
                        temperature = TemperaturePipe.celsiusToFahrenheit(measurement.temp);
                        break;
                }
                deviceSeriesMetadata.data.push({
                    x: newDate,
                    y: temperature,
                });
            }
            deviceTemperatureSeries.push(deviceSeriesMetadata);
        }
        return alignTemperatureChartSeries(deviceTemperatureSeries);
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
    private _prepareChartData(): void {
        const temperatureUnit = this.config.temperature_unit === 'celsius' ? 'C' : 'F';

        this.temperatureOptions = {
            chart: {
                animations: {
                    speed: 400,
                    animateGradually: {
                        enabled: false,
                    },
                },
                fontFamily: 'inherit',
                foreColor: 'inherit',
                width: '100%',
                height: '100%',
                parentHeightOffset: 0,
                type: 'area',
                sparkline: {
                    enabled: false,
                },
                redrawOnParentResize: true,
                redrawOnWindowResize: true,
                toolbar: {
                    show: false,
                },
            },
            colors: ['#667eea', '#9066ea', '#66c0ea', '#66ead2', '#d266ea', '#66ea90'],
            fill: {
                colors: ['#b2bef4', '#c7b2f4', '#b2dff4', '#b2f4e8', '#e8b2f4', '#b2f4c7'],
                opacity: 0.5,
                type: 'gradient',
            },
            legend: {
                show: true,
                position: 'bottom',
                horizontalAlign: 'left',
                fontSize: '12px',
                itemMargin: {
                    horizontal: 10,
                    vertical: 4,
                },
            },
            series: this._deviceDataTemperatureSeries(),
            stroke: {
                curve: this.config.line_stroke,
                width: 2,
            },
            markers: {
                size: 0,
                hover: {
                    sizeOffset: 4,
                },
            },
            dataLabels: {
                enabled: false,
            },
            tooltip: {
                theme: 'dark',
                shared: true,
                intersect: false,
                fixed: {
                    enabled: true,
                    position: 'topLeft',
                    offsetX: 12,
                    offsetY: 12,
                },
                custom: createTemperatureChartTooltip(this._document, this.config.temperature_unit, this.config.time_format),
            },
            xaxis: {
                type: 'datetime',
                tooltip: {
                    enabled: false,
                },
                labels: {
                    datetimeUTC: false,
                    style: {
                        fontSize: '11px',
                        colors: this.isDarkMode() ? '#9ca3af' : '#6b7280',
                    },
                    datetimeFormatter: {
                        year: 'yyyy',
                        month: "MMM 'yy",
                        day: 'dd MMM',
                        hour: this.config.time_format === '12' ? 'hh:mm tt' : 'HH:mm',
                    },
                },
            },
            yaxis: {
                labels: {
                    formatter: (value) => {
                        return `${Math.round(value)}${temperatureUnit}`;
                    },
                    style: {
                        fontSize: '11px',
                        colors: this.isDarkMode() ? '#9ca3af' : '#6b7280',
                    },
                },
                title: {
                    text: `Temperature (${temperatureUnit})`,
                    style: {
                        fontSize: '12px',
                        color: this.isDarkMode() ? '#9ca3af' : '#6b7280',
                    },
                },
            },
            grid: {
                borderColor: this.isDarkMode() ? '#374151' : '#e0e0e0',
                strokeDashArray: 4,
                yaxis: {
                    lines: {
                        show: true,
                    },
                },
                xaxis: {
                    lines: {
                        show: false,
                    },
                },
                padding: {
                    left: 10,
                    right: 10,
                },
            },
        };
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    deviceSummariesForHostGroup(hostGroupWWNs: string[]): DeviceSummaryModel[] {
        const deviceSummaries: DeviceSummaryModel[] = [];
        for (const wwn of hostGroupWWNs) {
            if (this.summaryData[wwn]) {
                deviceSummaries.push(this.summaryData[wwn]);
            }
        }
        return deviceSummaries;
    }

    filesystemHosts(): string[] {
        if (!this.filesystemSummaryData?.hosts) {
            return [];
        }
        return Object.keys(this.filesystemSummaryData.hosts).sort((left, right) => left.localeCompare(right));
    }

    filesystemsForHost(hostId: string): FilesystemCapacityModel[] {
        return [...(this.filesystemSummaryData?.filesystems?.[hostId] || [])].sort((left, right) => left.mount_point.localeCompare(right.mount_point));
    }

    filesystemStatusForHost(hostId: string): FilesystemHostStatusModel | null {
        return this.filesystemSummaryData?.hosts?.[hostId] || null;
    }

    hasFilesystemData(): boolean {
        return this.filesystemHosts().length > 0;
    }

    filesystemUsageClass(filesystem: FilesystemCapacityModel): string {
        if (filesystem.used_percent >= 90) {
            return 'bg-red-500';
        }
        if (filesystem.used_percent >= 80) {
            return 'bg-yellow-500';
        }
        return 'bg-green-500';
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
        const dialogRef = this.dialog.open(DashboardSettingsComponent, { width: '800px', maxWidth: '95vw' });

        dialogRef.afterClosed().subscribe();
    }

    onDeviceDeleted(_deviceId: string): void {
        this.reloadAfterPageItemRemoval();
    }

    onDeviceArchived(_deviceId: string): void {
        this.reloadAfterPageItemRemoval();
    }

    onDeviceUnarchived(_deviceId: string): void {
        this.reloadAfterPageItemRemoval();
    }

    private reloadAfterPageItemRemoval(): void {
        this.loadSummaryPage(this.pagination.page);
    }

    toggleArchived(): void {
        this.showArchived = !this.showArchived;
        this.loadSummaryPage(1);
    }

    onPageChange(event: PageEvent): void {
        const pageSize = event.pageSize as DashboardHostPageSize;
        if (pageSize !== this.pagination.page_size) {
            this.config = { ...this.config, dashboard_host_page_size: pageSize };
            this._configService.config = { dashboard_host_page_size: pageSize };
        }
        this.loadSummaryPage(event.pageIndex + 1, pageSize);
    }

    applyHostSearch(): void {
        this.appliedHostSearch = this.hostSearch.trim();
        this.loadSummaryPage(1);
    }

    clearHostSearch(): void {
        this.hostSearch = '';
        this.appliedHostSearch = '';
        this.loadSummaryPage(1);
    }

    private loadSummaryPage(page: number, pageSize?: DashboardHostPageSize): void {
        const effectivePageSize = pageSize ?? this.config?.dashboard_host_page_size ?? 10;
        this._dashboardService
            .getSummaryPage({
                page,
                pageSize: effectivePageSize,
                groupBy: 'host',
                archived: this.showArchived,
                sort: this.config?.dashboard_sort,
                display: this.config?.dashboard_display,
                hostSearch: this.appliedHostSearch,
            })
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((response) => {
                if (response.pagination.total_pages > 0 && page > response.pagination.total_pages) {
                    this.loadSummaryPage(response.pagination.total_pages, effectivePageSize);
                }
            });
    }

    dashboardColumns(): DashboardColumns {
        const configuredColumns = this.config?.dashboard_columns;
        if (configuredColumns === 3 || configuredColumns === 4 || configuredColumns === 5) {
            return configuredColumns;
        }
        return 2;
    }

    dashboardDensity(): DashboardDensity {
        return this.config?.dashboard_density === 'compact' ? 'compact' : 'comfortable';
    }

    dashboardGridClass(): string {
        return `dashboard-device-grid--cols-${this.dashboardColumns()}`;
    }

    isCompactDashboard(): boolean {
        return this.dashboardDensity() === 'compact';
    }

    filteredTemperatureDevices(): TemperatureDeviceOption[] {
        const search = this.temperatureDeviceSearch.trim().toLowerCase();
        if (!search) {
            return this.temperatureDevices;
        }
        return this.temperatureDevices.filter((device) => {
            return [device.host_id, device.label, device.device_name, device.model_name, device.serial_number].some((value) => value?.toLowerCase().includes(search));
        });
    }

    temperatureDeviceTitle(device?: TemperatureDeviceOption): string {
        if (!device) {
            return 'Unknown drive';
        }
        const title = device.label || [device.device_name ? `/dev/${device.device_name.replace(/^\/dev\//, '')}` : '', device.model_name].filter(Boolean).join(' - ');
        return device.host_id ? `${device.host_id}: ${title || device.serial_number || device.device_id}` : title || device.serial_number || device.device_id;
    }

    isTemperatureDeviceSelected(deviceID: string): boolean {
        return this.temperatureSelection.has(deviceID);
    }

    temperatureSelectionDisabled(deviceID: string): boolean {
        return !this.temperatureSelection.has(deviceID) && this.temperatureSelection.ids.length >= this.temperatureSelection.maxSelected;
    }

    toggleTemperatureDevice(deviceID: string): void {
        this.temperatureSelection.toggle(deviceID);
        this.loadSelectedTemperatureHistory();
    }

    /*
    DURATION_KEY_DAY    = "day"
    DURATION_KEY_WEEK    = "week"
    DURATION_KEY_MONTH   = "month"
    DURATION_KEY_YEAR    = "year"
    DURATION_KEY_FOREVER = "forever"
     */

    changeSummaryTempDuration(durationKey: string): void {
        this.tempDurationKey = durationKey;
        this.loadSelectedTemperatureHistory();
    }

    private loadSelectedTemperatureHistory(): void {
        const selectedDeviceIDs = this.temperatureSelection.ids;
        const requestID = ++this.temperatureRequestID;
        if (selectedDeviceIDs.length === 0) {
            this.temperatureHistory = {};
            this._updateTemperatureChartSeries([]);
            this._changeDetectorRef.markForCheck();
            return;
        }
        this._dashboardService
            .getSummaryTempData(this.tempDurationKey, selectedDeviceIDs)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((tempHistoryData) => {
                if (requestID !== this.temperatureRequestID) {
                    return;
                }
                this.temperatureHistory = tempHistoryData;
                this._updateTemperatureChartSeries(this._deviceDataTemperatureSeries());
                this._changeDetectorRef.markForCheck();
            });
    }

    private _updateTemperatureChartSeries(series: TemperatureChartSeries[]): void {
        if (!this.temperatureOptions) {
            return;
        }
        this.temperatureOptions = { ...this.temperatureOptions, series };
    }

    /**
     * Track by function for ngFor loops
     *
     * @param index
     * @param item
     */
    trackByFn(index: number, item: any): any {
        return item.id || index;
    }

    runCollectors(): void {
        if (this.isTriggering) {
            return;
        }

        this.isTriggering = true;
        this._dashboardService.runCollectors().subscribe(
            () => {
                this.countdown = 15;
                this._changeDetectorRef.markForCheck();

                const interval = setInterval(() => {
                    this.countdown--;
                    this._changeDetectorRef.markForCheck();

                    if (this.countdown <= 0) {
                        clearInterval(interval);
                        window.location.reload();
                    }
                }, 1000);
            },
            (err) => {
                this.isTriggering = false;
                this._changeDetectorRef.markForCheck();
                console.error('Failed to trigger collectors', err);
            }
        );
    }
}
