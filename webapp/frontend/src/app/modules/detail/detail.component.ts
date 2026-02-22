import humanizeDuration from 'humanize-duration';
import {AfterViewInit, Component, Inject, LOCALE_ID, OnDestroy, OnInit, ViewChild} from '@angular/core';
import {ApexOptions} from 'ng-apexcharts';
import {AppConfig} from 'app/core/config/app.config';
import {DetailService} from './detail.service';
import {DetailSettingsComponent} from 'app/layout/common/detail-settings/detail-settings.component';
import {AttributeHistoryDialogComponent, AttributeHistoryData} from 'app/layout/common/attribute-history-dialog/attribute-history-dialog.component';
import {MatDialog as MatDialog} from '@angular/material/dialog';
import {MatSort} from '@angular/material/sort';
import {MatTableDataSource as MatTableDataSource} from '@angular/material/table';
import {Subject} from 'rxjs';
import {ScrutinyConfigService} from 'app/core/config/scrutiny-config.service';
import {animate, state, style, transition, trigger} from '@angular/animations';
import {formatDate} from '@angular/common';
import {takeUntil} from 'rxjs/operators';
import {DeviceModel} from 'app/core/models/device-model';
import {SmartModel} from 'app/core/models/measurements/smart-model';
import {SmartAttributeModel} from 'app/core/models/measurements/smart-attribute-model';
import {AttributeMetadataModel} from 'app/core/models/thresholds/attribute-metadata-model';
import {DeviceStatusPipe} from 'app/shared/device-status.pipe';
import {PerformanceModel, PerformanceBaselineModel, PerformanceResponseWrapper} from 'app/core/models/measurements/performance-model';
import {LatencyPipe} from 'app/shared/latency.pipe';
import {FileSizePipe} from 'app/shared/file-size.pipe';
import {AttributeOverrideService} from 'app/core/config/attribute-override.service';
import {AttributeOverride, OverrideProtocol} from 'app/core/config/app.config';

// from Constants.go - these must match
const AttributeStatusPassed = 0
const AttributeStatusFailedSmart = 1
const AttributeStatusWarningScrutiny = 2
const AttributeStatusFailedScrutiny = 4


@Component({
    selector: 'detail',
    templateUrl: './detail.component.html',
    styleUrls: ['./detail.component.scss'],
    animations: [
        trigger('detailExpand', [
            state('collapsed', style({ height: '0px', minHeight: '0' })),
            state('expanded', style({ height: '*' })),
            transition('expanded <=> collapsed', animate('225ms cubic-bezier(0.4, 0.0, 0.2, 1)')),
        ]),
    ],
    standalone: false
})

export class DetailComponent implements OnInit, AfterViewInit, OnDestroy {

    /**
     * Constructor
     *
     * @param {DetailService} _detailService
     * @param {MatDialog} dialog
     * @param {ScrutinyConfigService} _configService
     * @param {string} locale
     */
    constructor(
        private _detailService: DetailService,
        public dialog: MatDialog,
        private _configService: ScrutinyConfigService,
        private _overrideService: AttributeOverrideService,
        @Inject(LOCALE_ID) public locale: string
    ) {
        // Set the private defaults
        this._unsubscribeAll = new Subject();

        // Set the defaults
        this.smartAttributeDataSource = new MatTableDataSource();
        // this.recentTransactionsTableColumns = ['status', 'id', 'name', 'value', 'worst', 'thresh'];
        this.smartAttributeTableColumns = ['status', 'id', 'name', 'value', 'worst', 'thresh', 'ideal', 'failure', 'history'];

        this.systemPrefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;

    }

    config: AppConfig;

    activeOverrides: AttributeOverride[] = [];
    private _overrideMap: Map<string, AttributeOverride> = new Map();
    onlyCritical = true;
    // data: any;
    expandedAttribute: SmartAttributeModel | null;

    // User preference for attribute value display: "scrutiny" (default), "raw", or "normalized"
    displayMode: 'scrutiny' | 'raw' | 'normalized' = 'scrutiny';

    metadata: { [p: string]: AttributeMetadataModel } | { [p: number]: AttributeMetadataModel };
    device: DeviceModel;
    // tslint:disable-next-line:variable-name
    smart_results: SmartModel[];

    commonSparklineOptions: Partial<ApexOptions>;
    smartAttributeDataSource: MatTableDataSource<SmartAttributeModel>;
    smartAttributeTableColumns: string[];

    @ViewChild('smartAttributeTable', {read: MatSort})
    smartAttributeTableMatSort: MatSort;

    // Performance benchmarks
    performanceHistory: PerformanceModel[] = [];
    performanceBaseline: PerformanceBaselineModel | null = null;
    hasPerformanceData = false;
    performanceEverLoaded = false;
    perfDurationKey = 'week';
    performanceLoading = false;
    hasThroughputData = false;
    hasIopsData = false;
    hasLatencyData = false;
    hasEnoughSamplesForCharts = false;
    throughputChartOptions: Partial<ApexOptions>;
    iopsChartOptions: Partial<ApexOptions>;
    latencyChartOptions: Partial<ApexOptions>;

    // Private
    private _unsubscribeAll: Subject<void>;
    private systemPrefersDark: boolean;

    readonly humanizeDuration = humanizeDuration;

    deviceStatusForModelWithThreshold = DeviceStatusPipe.deviceStatusForModelWithThreshold
    // -----------------------------------------------------------------------------------------------------
    // @ Lifecycle hooks
    // -----------------------------------------------------------------------------------------------------

    /**
     * On init
     */
    ngOnInit(): void {
        // Subscribe to config changes
        this._configService.config$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((config: AppConfig) => {

                this.config = config;
            });

        // Load attribute overrides
        this._overrideService.getOverrides()
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(overrides => {
                this.activeOverrides = overrides;
                this._buildOverrideMap();
            });

        // Get the data
        this._detailService.data$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((respWrapper) => {

                // Store the data
                // this.data = data;
                this.device = respWrapper.data.device;
                this.smart_results = respWrapper.data.smart_results
                this.metadata = respWrapper.metadata;

                // Initialize display mode from device preference (default to 'scrutiny')
                this.displayMode = (this.device.smart_display_mode as 'scrutiny' | 'raw' | 'normalized') || 'scrutiny';

                // Store the table data
                this.smartAttributeDataSource.data = this._generateSmartAttributeTableDataSource(this.smart_results);

                // Prepare the chart data
                this._prepareChartData();

                // Load performance data (lazy, non-blocking)
                this._loadPerformanceData(this.device.wwn, this.perfDurationKey);
            });
    }

    /**
     * After view init
     */
    ngAfterViewInit(): void {
        this.smartAttributeDataSource.sortingDataAccessor = (data, sortHeaderId) => {
            switch (sortHeaderId) {
                case 'id': return data.attribute_id;
                case 'name': return this.getAttributeName(data);
                case 'failure': return data.failure_rate ?? 0;
                case 'worst': return this.getAttributeWorst(data) || 0;
                case 'thresh': return this.getAttributeThreshold(data) || 0;
                case 'ideal': return this.getAttributeIdeal(data);
                default: return data[sortHeaderId];
            }
        };

        // Make the data source sortable
        this.smartAttributeDataSource.sort = this.smartAttributeTableMatSort;
    }

    /**
     * On destroy
     */
    ngOnDestroy(): void {
        // Unsubscribe from all subscriptions
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Private methods
    // -----------------------------------------------------------------------------------------------------

    getAttributeStatusName(attributeStatus: number): string {
        // tslint:disable:no-bitwise

        if (attributeStatus === AttributeStatusPassed) {
            return 'passed'

        } else if ((attributeStatus & AttributeStatusFailedScrutiny) !== 0 || (attributeStatus & AttributeStatusFailedSmart) !== 0) {
            return 'failed'
        } else if ((attributeStatus & AttributeStatusWarningScrutiny) !== 0) {
            return 'warn'
        }
        return ''
        // tslint:enable:no-bitwise
    }

    getAttributeScrutinyStatusName(attributeStatus: number): string {
        // tslint:disable:no-bitwise
        if ((attributeStatus & AttributeStatusFailedScrutiny) !== 0) {
            return 'failed'
        } else if ((attributeStatus & AttributeStatusWarningScrutiny) !== 0) {
            return 'warn'
        } else {
            return 'passed'
        }
        // tslint:enable:no-bitwise
    }

    getAttributeSmartStatusName(attributeStatus: number): string {
        // tslint:disable:no-bitwise
        if ((attributeStatus & AttributeStatusFailedSmart) !== 0) {
            return 'failed'
        } else {
            return 'passed'
        }
        // tslint:enable:no-bitwise
    }


    getAttributeName(attributeData: SmartAttributeModel): string {
        const attributeMetadata = this.metadata[attributeData.attribute_id]
        if (!attributeMetadata) {
            return 'Unknown Attribute Name'
        } else {
            return attributeMetadata.display_name
        }
    }

    getAttributeDescription(attributeData: SmartAttributeModel): string {
        const attributeMetadata = this.metadata[attributeData.attribute_id]
        if (!attributeMetadata) {
            return 'Unknown'
        } else {
            return attributeMetadata.description
        }
    }

    getAttributeValue(attributeData: SmartAttributeModel): number {
        // For non-ATA devices, always return value (no raw/normalized distinction)
        if (!this.isAta()) {
            return attributeData.value
        }

        const attributeMetadata = this.metadata[attributeData.attribute_id]

        // EXCEPTION: Transformed attributes (like temperature) always use transformed value
        // because raw value is packed bytes that aren't human-readable
        if (attributeMetadata?.display_type === 'transformed' && attributeData.transformed_value) {
            return attributeData.transformed_value
        }

        // Apply display mode preference
        switch (this.displayMode) {
            case 'raw':
                // Device statistics (devstat_*) don't have raw_value, use value instead
                return attributeData.raw_value ?? attributeData.value
            case 'normalized':
                return attributeData.value
            case 'scrutiny':
            default:
                // Current behavior - use metadata display_type
                if (attributeMetadata?.display_type === 'raw') {
                    return attributeData.raw_value ?? attributeData.value
                }
                return attributeData.value
        }
    }

    getAttributeValueType(attributeData: SmartAttributeModel): string {
        if (this.isAta()) {
            const attributeMetadata = this.metadata[attributeData.attribute_id]
            if (!attributeMetadata) {
                return ''
            } else {
                return attributeMetadata.display_type
            }
        } else {
            return ''
        }
    }

    getAttributeIdeal(attributeData: SmartAttributeModel): string {
        if (this.isAta()) {
            return this.metadata[attributeData.attribute_id]?.display_type === 'raw' ? this.metadata[attributeData.attribute_id]?.ideal : ''
        } else {
            return this.metadata[attributeData.attribute_id]?.ideal
        }
    }

    getAttributeWorst(attributeData: SmartAttributeModel): number | string {
        const attributeMetadata = this.metadata[attributeData.attribute_id]
        if (!attributeMetadata) {
            return attributeData.worst
        } else {
            return attributeMetadata?.display_type === 'normalized' ? attributeData.worst : ''
        }
    }

    getAttributeThreshold(attributeData: SmartAttributeModel): number | string {
        if (this.isAta()) {
            const attributeMetadata = this.metadata[attributeData.attribute_id]
            if (!attributeMetadata || attributeMetadata.display_type === 'normalized') {
                return attributeData.thresh
            } else {
                return ''
            }
        } else {
            return (attributeData.thresh === -1 ? '' : attributeData.thresh)
        }
    }

    getAttributeCritical(attributeData: SmartAttributeModel): boolean {
        return this.metadata[attributeData.attribute_id]?.critical
    }

    getHiddenAttributes(): number {
        if (!this.smart_results || this.smart_results.length === 0) {
            return 0
        }

        let attributesLength = 0
        const attributes = this.smart_results[0]?.attrs
        if (attributes) {
            attributesLength = Object.keys(attributes).length
        }

        return attributesLength - this.smartAttributeDataSource.data.length
    }

    getSSDWearoutValue(): number | null {
        if (!this.smart_results || this.smart_results.length === 0) {
            return null;
        }

        const attrs = this.smart_results[0]?.attrs;
        if (!attrs) {
            return null;
        }

        // Check in order of preference: 177 (Samsung/Crucial), 233 (Intel), 231 (Life Left), 232 (Endurance)
        const wearoutAttr = attrs['177'] || attrs['233'] || attrs['231'] || attrs['232'];
        if (wearoutAttr) {
            return wearoutAttr.value; // Normalized value (0-100)
        }

        return null;
    }

    getSSDPercentageUsed(): number | null {
        if (!this.smart_results || this.smart_results.length === 0) {
            return null;
        }

        const attrs = this.smart_results[0]?.attrs;
        if (!attrs) {
            return null;
        }

        // Check for percentage_used (NVMe) or devstat_7_8 (ATA)
        const percentageUsedAttr = attrs['percentage_used'] || attrs['devstat_7_8'];
        if (percentageUsedAttr) {
            // For percentage_used, use value; for devstat_7_8, use raw_value if available, otherwise value
            return percentageUsedAttr.raw_value !== undefined ? percentageUsedAttr.raw_value : percentageUsedAttr.value;
        }

        return null;
    }

    isAta(): boolean {
        return this.device?.device_protocol === 'ATA'
    }

    isScsi(): boolean {
        return this.device?.device_protocol === 'SCSI'
    }

    isNvme(): boolean {
        return this.device?.device_protocol === 'NVMe'
    }

    /**
     * Check if collector version is older than server version
     */
    isCollectorOutdated(): boolean {
        const collectorVersion = this.device?.collector_version;
        const serverVersion = this.config?.server_version;

        if (!collectorVersion || !serverVersion) {
            return false;
        }

        // Simple semver comparison - works when format is consistent (x.y.z)
        return collectorVersion < serverVersion;
    }

    /**
     * Set the SMART attribute display mode and persist to backend
     * @param mode Display mode: "scrutiny", "raw", or "normalized"
     */
    setDisplayMode(mode: 'scrutiny' | 'raw' | 'normalized'): void {
        this.displayMode = mode;

        // Clear cached chart data to force regeneration with new display mode values
        if (this.smart_results?.length > 0) {
            const attrs = this.smart_results[0]?.attrs;
            if (attrs) {
                for (const attrId in attrs) {
                    delete attrs[attrId].chartData;
                }
            }
        }

        // Regenerate table data with new display mode
        this.smartAttributeDataSource.data = this._generateSmartAttributeTableDataSource(this.smart_results);

        // Persist preference to backend
        this._detailService.setSmartDisplayMode(this.device.wwn, mode).subscribe();
    }

    private _generateSmartAttributeTableDataSource(smartResults: SmartModel[]): SmartAttributeModel[] {
        const smartAttributeDataSource: SmartAttributeModel[] = [];

        if (smartResults.length === 0) {
            return smartAttributeDataSource
        }
        const latestSmartResult = smartResults[0];
        let attributes: { [p: string]: SmartAttributeModel } = {}
        if (this.isScsi()) {
            this.smartAttributeTableColumns = ['status', 'name', 'value', 'thresh', 'history', 'actions'];
            attributes = latestSmartResult.attrs
        } else if (this.isNvme()) {
            this.smartAttributeTableColumns = ['status', 'name', 'value', 'thresh', 'ideal', 'history', 'actions'];
            attributes = latestSmartResult.attrs
        } else {
            // ATA
            attributes = latestSmartResult.attrs
            // Only show 'worst' column in scrutiny or normalized mode (worst is meaningless for raw values)
            if (this.displayMode === 'raw') {
                this.smartAttributeTableColumns = ['status', 'id', 'name', 'value', 'thresh', 'ideal', 'failure', 'history', 'actions'];
            } else {
                this.smartAttributeTableColumns = ['status', 'id', 'name', 'value', 'worst', 'thresh', 'ideal', 'failure', 'history', 'actions'];
            }
        }

        for (const attrId in attributes) {
            const attr = attributes[attrId]

            // chart history data
            if (!attr.chartData) {


                const attrHistory = []
                for (const smartResult of smartResults) {
                    // attrHistory.push(this.getAttributeValue(smart_result.attrs[attrId]))

                    const chartDatapoint = {
                        x: formatDate(smartResult.date, 'MMMM dd, yyyy - HH:mm', this.locale),
                        y: this.getAttributeValue(smartResult.attrs[attrId])
                    }
                    const attributeStatusName = this.getAttributeStatusName(smartResult.attrs[attrId].status)
                    if (attributeStatusName === 'failed') {
                        chartDatapoint['strokeColor'] = '#F05252'
                        chartDatapoint['fillColor'] = '#F05252'
                    } else if (attributeStatusName === 'warn') {
                        chartDatapoint['strokeColor'] = '#C27803'
                        chartDatapoint['fillColor'] = '#C27803'
                    }
                    attrHistory.push(chartDatapoint)
                }

                // var rawHistory = (attr.history || []).map(hist_attr => this.getAttributeValue(hist_attr)).reverse()
                // rawHistory.push(this.getAttributeValue(attr))

                attributes[attrId].chartData = [
                    {
                        name: 'chart-line-sparkline',
                        // attrHistory needs to be reversed, so the newest data is on the right
                        // fixes #339
                        data: attrHistory.reverse()
                    }
                ]
            }
            // determine when to include the attributes in table.

            if (!this.onlyCritical || this.onlyCritical && this.metadata[attr.attribute_id]?.critical || attr.value < attr.thresh) {
                smartAttributeDataSource.push(attr)
            }
        }
        return smartAttributeDataSource
    }

    /**
     * Prepare the chart data from the data
     *
     * @private
     */
    private _prepareChartData(): void {

        // Account balance
        this.commonSparklineOptions = {
            chart: {
                type: 'bar',
                width: 100,
                height: 25,
                sparkline: {
                    enabled: true
                },
                animations: {
                    enabled: false
                }
            },
            tooltip: {
                enabled: false
            },
            stroke: {
                width: 2,
                colors: ['#667EEA']
            }
        };
    }

    private determineTheme(config: AppConfig): string {
        if (config.theme === 'system') {
            return this.systemPrefersDark ? 'dark' : 'light'
        } else {
            return config.theme
        }
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    toHex(decimalNumb: number | string): string {
        // Device statistics use string-based IDs like "devstat_7_8"
        // Only convert numeric values to hex
        const num = Number(decimalNumb);
        if (isNaN(num)) {
            return '';
        }
        return '0x' + num.toString(16).padStart(2, '0').toUpperCase()
    }

    formatAttributeId(attributeId: number | string): string {
        // For string-based IDs (device statistics), just return the ID
        if (typeof attributeId === 'string' && isNaN(Number(attributeId))) {
            return attributeId;
        }
        // For numeric IDs, show both decimal and hex
        const hex = this.toHex(attributeId);
        return hex ? `${attributeId} (${hex})` : `${attributeId}`;
    }

    toggleOnlyCritical(): void {
        this.onlyCritical = !this.onlyCritical
        this.smartAttributeDataSource.data = this._generateSmartAttributeTableDataSource(this.smart_results);
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Attribute override methods
    // -----------------------------------------------------------------------------------------------------

    getOverrideForAttribute(attrId: string | number): AttributeOverride | null {
        const protocol = this.device?.device_protocol;
        const attrIdStr = String(attrId);
        // Prefer device-specific override, fall back to global
        return this._overrideMap.get(`${protocol}:${attrIdStr}:${this.device?.wwn}`)
            || this._overrideMap.get(`${protocol}:${attrIdStr}:`)
            || null;
    }

    private _buildOverrideMap(): void {
        this._overrideMap.clear();
        // Insert global overrides first, then device-specific so they take priority
        for (const o of this.activeOverrides) {
            const key = `${o.protocol}:${o.attribute_id}:${o.wwn || ''}`;
            this._overrideMap.set(key, o);
        }
    }

    ignoreAttribute(attr: SmartAttributeModel): void {
        const override: AttributeOverride = {
            protocol: this.device.device_protocol as OverrideProtocol,
            attribute_id: String(attr.attribute_id),
            wwn: this.device.wwn,
            action: 'ignore'
        };
        this._overrideService.saveOverride(override)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe({
                next: () => this._refreshAfterOverrideChange(),
                error: (err) => console.error('Failed to save override:', err)
            });
    }

    forcePassedAttribute(attr: SmartAttributeModel): void {
        const override: AttributeOverride = {
            protocol: this.device.device_protocol as OverrideProtocol,
            attribute_id: String(attr.attribute_id),
            wwn: this.device.wwn,
            action: 'force_status',
            status: 'passed'
        };
        this._overrideService.saveOverride(override)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe({
                next: () => this._refreshAfterOverrideChange(),
                error: (err) => console.error('Failed to save override:', err)
            });
    }

    removeOverride(attr: SmartAttributeModel): void {
        const override = this.getOverrideForAttribute(attr.attribute_id);
        if (override?.id) {
            this._overrideService.deleteOverride(override.id)
                .pipe(takeUntil(this._unsubscribeAll))
                .subscribe({
                    next: () => this._refreshAfterOverrideChange(),
                    error: (err) => console.error('Failed to remove override:', err)
                });
        }
    }

    private _refreshAfterOverrideChange(): void {
        this._overrideService.getOverrides()
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(overrides => {
                this.activeOverrides = overrides;
                this._buildOverrideMap();
            });
        this._detailService.getData(this.device.wwn)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe();
    }

    openSettingsDialog(): void {
        if (!this.device) return;

        const dialogRef = this.dialog.open(DetailSettingsComponent, {
            width: '600px',
            data: {
                curMuted: this.device.muted,
                curLabel: this.device.label
            },
        });

        dialogRef.afterClosed().subscribe((result: undefined | null | { muted: boolean, label: string }) => {
            if (!result || !this.device) return;

            const promises: Promise<any>[] = [];

            if (result.muted !== this.device.muted) {
                promises.push(this._detailService.setMuted(this.device.wwn, result.muted).toPromise());
            }

            if (result.label !== this.device.label) {
                promises.push(this._detailService.setLabel(this.device.wwn, result.label).toPromise());
            }

            if (promises.length > 0) {
                Promise.all(promises).then(() => {
                    return this._detailService.getData(this.device.wwn).toPromise();
                });
            }
        });
    }

    /**
     * Track by function for ngFor loops
     *
     * @param index
     * @param item
     */
    trackByFn(index: number, item: any): any {
        return index;
        // return item.id || index;
    }

    /**
     * Convert raw attribute value to TB based on the attribute name from smartctl.
     * Different vendors use different units for attributes 241/242/246/249:
     * - Intel/Crucial/Micron SSDs: 32MiB units (name contains "32MiB")
     * - Some SSDs: GiB units (name contains "GiB" or "1GiB")
     * - Crucial/Micron: Host Sector Writes (attribute 246)
     * - Budget SSDs (e.g., Patriot): Use 32 MiB units despite "Total_LBAs" name
     * - Default: LBA units (multiply by logical block size)
     */
    private convertToTB(rawValue: number, attrName: string | undefined): number {
        const TB = 1024 * 1024 * 1024 * 1024;
        const ONE_GB = 1024 * 1024 * 1024;
        const blockSize = this.smart_results[0]?.logical_block_size || 512;

        if (!attrName) {
            // No name available, assume LBA units
            return (rawValue * blockSize) / TB;
        }

        if (attrName.includes('32MiB')) {
            // Intel, Crucial/Micron, InnoDisk SSDs use 32 MiB per unit
            return (rawValue * 32 * 1024 * 1024) / TB;
        } else if (attrName.includes('GiB') || attrName.includes('1GiB')) {
            // Some SSDs report in GiB units (including attribute 249)
            return rawValue / 1024;
        } else if (attrName.includes('Sector') || attrName.includes('Host_Writes') || attrName.includes('Host_Reads')) {
            // Host Sector Writes/Reads (attribute 246) - sectors = LBAs
            return (rawValue * blockSize) / TB;
        }

        // HEURISTIC: Detect budget SSDs (e.g., Patriot Burst Elite with Silicon Motion controller)
        // that use 32 MiB units despite generic "Total_LBAs" attribute name.
        // If raw value * blockSize < 1 GB, the value is too small to be actual LBAs.
        // Verified by cross-referencing with ATA Device Statistics (devstat_1_24).
        const bytesIfLBA = rawValue * blockSize;
        if (bytesIfLBA < ONE_GB && rawValue > 0) {
            return (rawValue * 32 * 1024 * 1024) / TB;
        }

        // Default: assume LBA units, multiply by logical block size
        return (rawValue * blockSize) / TB;
    }

    /**
     * Calculate TBs written from LBAs (ATA) or data units (NVMe)
     * Uses attribute name from smartctl to determine correct unit conversion
     * Checks multiple ATA attributes in order of preference:
     * - 241: Total LBAs Written (standard)
     * - 246: Total Host Sector Writes (Crucial/Micron)
     * - 249: NAND Writes in 1GiB (some vendors)
     */
    getTBsWritten(): number | null {
        if (!this.smart_results || this.smart_results.length === 0) {
            return null;
        }

        const attrs = this.smart_results[0]?.attrs;
        if (!attrs) {
            return null;
        }

        const TB = 1024 * 1024 * 1024 * 1024;
        const blockSize = this.smart_results[0]?.logical_block_size || 512;

        // PRIORITY 1: ATA Device Statistics - devstat_1_24 (Logical Sectors Written)
        // Standardized logical sector units per ACS spec, no unit guessing needed
        const devstatWritten = attrs['devstat_1_24'];
        if (devstatWritten?.value != null && devstatWritten.value > 0) {
            return (devstatWritten.value * blockSize) / TB;
        }

        // PRIORITY 2: ATA SMART attributes with name-based unit detection
        // 241 = Total LBAs Written (standard)
        const ataAttr = attrs['241'];
        if (ataAttr?.raw_value != null) {
            return this.convertToTB(ataAttr.raw_value, ataAttr.name);
        }

        // 246 = Total Host Sector Writes (Crucial/Micron)
        const hostSectorAttr = attrs['246'];
        if (hostSectorAttr?.raw_value != null) {
            return this.convertToTB(hostSectorAttr.raw_value, hostSectorAttr.name);
        }

        // 249 = NAND Writes in 1GiB (some vendors)
        const nandWriteAttr = attrs['249'];
        if (nandWriteAttr?.raw_value != null) {
            return this.convertToTB(nandWriteAttr.raw_value, nandWriteAttr.name);
        }

        // NVMe: data_units_written is in 512KB (512 * 1000 bytes) units per NVMe spec
        const nvmeAttr = attrs['data_units_written'];
        if (nvmeAttr?.value != null) {
            return (nvmeAttr.value * 512 * 1000) / (1024 * 1024 * 1024 * 1024);
        }

        return null;
    }

    /**
     * Calculate TBs read from LBAs (ATA) or data units (NVMe)
     * Uses attribute name from smartctl to determine correct unit conversion
     * Checks multiple ATA attributes in order of preference:
     * - 242: Total LBAs Read (standard)
     * - 244: Total LBAs Read Expanded (upper bytes for large values)
     */
    getTBsRead(): number | null {
        if (!this.smart_results || this.smart_results.length === 0) {
            return null;
        }

        const attrs = this.smart_results[0]?.attrs;
        if (!attrs) {
            return null;
        }

        const TB = 1024 * 1024 * 1024 * 1024;
        const blockSize = this.smart_results[0]?.logical_block_size || 512;

        // PRIORITY 1: ATA Device Statistics - devstat_1_40 (Logical Sectors Read)
        // Standardized logical sector units per ACS spec, no unit guessing needed
        const devstatRead = attrs['devstat_1_40'];
        if (devstatRead?.value != null && devstatRead.value > 0) {
            return (devstatRead.value * blockSize) / TB;
        }

        // PRIORITY 2: ATA SMART attributes with name-based unit detection
        // 242 = Total LBAs Read (standard)
        const ataAttr = attrs['242'];
        if (ataAttr?.raw_value != null) {
            return this.convertToTB(ataAttr.raw_value, ataAttr.name);
        }

        // 244 = Total LBAs Read Expanded (for drives with large read counts)
        const expandedAttr = attrs['244'];
        if (expandedAttr?.raw_value != null) {
            return this.convertToTB(expandedAttr.raw_value, expandedAttr.name);
        }

        // NVMe: data_units_read is in 512KB (512 * 1000 bytes) units per NVMe spec
        const nvmeAttr = attrs['data_units_read'];
        if (nvmeAttr?.value != null) {
            return (nvmeAttr.value * 512 * 1000) / (1024 * 1024 * 1024 * 1024);
        }

        return null;
    }

    openHistoryDialog(attribute: SmartAttributeModel, event: Event): void {
        event.stopPropagation(); // Prevent row expansion when clicking sparkline
        const dialogData: AttributeHistoryData = {
            attributeName: this.getAttributeName(attribute),
            chartData: attribute.chartData,
            isDark: this.determineTheme(this.config) === 'dark'
        };
        this.dialog.open(AttributeHistoryDialogComponent, {
            width: '600px',
            data: dialogData
        });
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Performance benchmarks
    // -----------------------------------------------------------------------------------------------------

    private _loadPerformanceData(wwn: string, duration: string): void {
        this.performanceLoading = true;
        this._detailService.getPerformanceData(wwn, duration)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe({
                next: (resp: PerformanceResponseWrapper) => {
                    this.performanceLoading = false;
                    if (resp.success && resp.data?.history && resp.data.history.length > 0) {
                        this.performanceHistory = resp.data.history;
                        this.performanceBaseline = resp.data.baseline;
                        this.hasPerformanceData = true;
                        this.performanceEverLoaded = true;
                        this.hasThroughputData = resp.data.history.some(p => p.seq_read_bw_bytes > 0 || p.seq_write_bw_bytes > 0);
                        this.hasIopsData = resp.data.history.some(p => p.rand_read_iops > 0 || p.rand_write_iops > 0);
                        this.hasLatencyData = resp.data.history.some(p => p.rand_read_lat_ns_avg > 0);
                        this.hasEnoughSamplesForCharts = resp.data.history.length >= 2;
                        this._preparePerformanceCharts();
                    } else {
                        this.hasPerformanceData = false;
                        this.performanceHistory = [];
                        this.performanceBaseline = null;
                        this.hasThroughputData = false;
                        this.hasIopsData = false;
                        this.hasLatencyData = false;
                        this.hasEnoughSamplesForCharts = false;
                    }
                },
                error: () => {
                    this.performanceLoading = false;
                    this.hasPerformanceData = false;
                }
            });
    }

    changePerformanceDuration(durationKey: string): void {
        this.perfDurationKey = durationKey;
        this._loadPerformanceData(this.device.wwn, durationKey);
    }

    getLatestPerformance(): PerformanceModel | null {
        return this.performanceHistory?.length > 0 ? this.performanceHistory[this.performanceHistory.length - 1] : null;
    }

    getBaselineDelta(currentValue: number, baselineValue: number, higherIsBetter: boolean): {
        percent: number; status: 'good' | 'warn' | 'bad' | 'neutral'
    } | null {
        if (!baselineValue || baselineValue === 0 || currentValue == null) {
            return null;
        }
        const pct = ((currentValue - baselineValue) / baselineValue) * 100;
        const delta = higherIsBetter ? pct : -pct;
        let status: 'good' | 'warn' | 'bad' | 'neutral';
        if (Math.abs(delta) < 5) {
            status = 'neutral';
        } else if (delta > 0) {
            status = 'good';
        } else if (delta > -15) {
            status = 'warn';
        } else {
            status = 'bad';
        }
        return { percent: Math.round(pct * 10) / 10, status };
    }

    private _preparePerformanceCharts(): void {
        const isDark = this.determineTheme(this.config) === 'dark';
        const labelColor = isDark ? '#9ca3af' : '#6b7280';
        const gridColor = isDark ? '#374151' : '#e0e0e0';
        const siUnits = this.config?.file_size_si_units;
        const fileSizePipe = new FileSizePipe();

        const baseChart = {
            animations: { speed: 400, animateGradually: { enabled: false } },
            fontFamily: 'inherit',
            foreColor: 'inherit',
            width: '100%',
            height: '100%',
            type: 'area' as const,
            sparkline: { enabled: false },
            toolbar: { show: false }
        };

        const baseGrid = {
            borderColor: gridColor,
            strokeDashArray: 4,
            yaxis: { lines: { show: true } },
            xaxis: { lines: { show: false } },
            padding: { left: 10, right: 10 }
        };

        const baseXAxis = {
            type: 'datetime' as const,
            labels: {
                datetimeUTC: false,
                style: { fontSize: '11px', colors: labelColor }
            }
        };

        // Throughput chart (sequential read/write)
        const readBwData = this.performanceHistory.map(p => ({
            x: new Date(p.date).getTime(), y: p.seq_read_bw_bytes
        }));
        const writeBwData = this.performanceHistory.map(p => ({
            x: new Date(p.date).getTime(), y: p.seq_write_bw_bytes
        }));

        this.throughputChartOptions = {
            chart: baseChart,
            colors: ['#667eea', '#e66a7a'],
            fill: { colors: ['#b2bef4', '#f4b2ba'], opacity: 0.5, type: 'gradient' },
            series: [
                { name: 'Sequential Read', data: readBwData },
                { name: 'Sequential Write', data: writeBwData }
            ],
            stroke: { curve: 'smooth', width: 2 },
            tooltip: {
                theme: 'dark', shared: true, intersect: false,
                x: { format: 'MMM dd, yyyy HH:mm' },
                y: { formatter: (val) => fileSizePipe.transform(val, siUnits) + '/s' }
            },
            xaxis: baseXAxis,
            yaxis: {
                labels: {
                    formatter: (val) => fileSizePipe.transform(val, siUnits) + '/s',
                    style: { fontSize: '11px', colors: labelColor }
                }
            },
            grid: baseGrid,
            legend: { show: true, position: 'top', horizontalAlign: 'right' }
        };

        // IOPS chart (random read/write + mixed)
        const readIopsData = this.performanceHistory.map(p => ({
            x: new Date(p.date).getTime(), y: p.rand_read_iops
        }));
        const writeIopsData = this.performanceHistory.map(p => ({
            x: new Date(p.date).getTime(), y: p.rand_write_iops
        }));
        const mixedSeries = this.performanceHistory.some(p => p.mixed_rw_iops > 0);
        const iopsSeries: any[] = [
            { name: 'Random Read', data: readIopsData },
            { name: 'Random Write', data: writeIopsData }
        ];
        const iopsColors = ['#667eea', '#e66a7a'];
        const iopsFillColors = ['#b2bef4', '#f4b2ba'];
        if (mixedSeries) {
            iopsSeries.push({
                name: 'Mixed R/W',
                data: this.performanceHistory.map(p => ({
                    x: new Date(p.date).getTime(), y: p.mixed_rw_iops
                }))
            });
            iopsColors.push('#66c0ea');
            iopsFillColors.push('#b2dff4');
        }

        this.iopsChartOptions = {
            chart: baseChart,
            colors: iopsColors,
            fill: { colors: iopsFillColors, opacity: 0.5, type: 'gradient' },
            series: iopsSeries,
            stroke: { curve: 'smooth', width: 2 },
            tooltip: {
                theme: 'dark', shared: true, intersect: false,
                x: { format: 'MMM dd, yyyy HH:mm' },
                y: { formatter: (val) => val != null ? val.toLocaleString() + ' IOPS' : '--' }
            },
            xaxis: baseXAxis,
            yaxis: {
                labels: {
                    formatter: (val) => val >= 1000 ? (val / 1000).toFixed(1) + 'K' : String(Math.round(val)),
                    style: { fontSize: '11px', colors: labelColor }
                }
            },
            grid: baseGrid,
            legend: { show: true, position: 'top', horizontalAlign: 'right' }
        };

        // Latency chart (read avg / p95 / p99)
        const readLatAvgData = this.performanceHistory.map(p => ({
            x: new Date(p.date).getTime(), y: p.rand_read_lat_ns_avg
        }));
        const readLatP95Data = this.performanceHistory.map(p => ({
            x: new Date(p.date).getTime(), y: p.rand_read_lat_ns_p95
        }));
        const readLatP99Data = this.performanceHistory.map(p => ({
            x: new Date(p.date).getTime(), y: p.rand_read_lat_ns_p99
        }));

        this.latencyChartOptions = {
            chart: baseChart,
            colors: ['#667eea', '#e6a23c', '#e66a7a'],
            fill: { colors: ['#b2bef4', '#f4dbb2', '#f4b2ba'], opacity: 0.5, type: 'gradient' },
            series: [
                { name: 'Avg Latency', data: readLatAvgData },
                { name: 'P95 Latency', data: readLatP95Data },
                { name: 'P99 Latency', data: readLatP99Data }
            ],
            stroke: { curve: 'smooth', width: 2 },
            tooltip: {
                theme: 'dark', shared: true, intersect: false,
                x: { format: 'MMM dd, yyyy HH:mm' },
                y: { formatter: (val) => LatencyPipe.formatLatency(val) }
            },
            xaxis: baseXAxis,
            yaxis: {
                labels: {
                    formatter: (val) => LatencyPipe.formatLatency(val, 0),
                    style: { fontSize: '11px', colors: labelColor }
                }
            },
            grid: baseGrid,
            legend: { show: true, position: 'top', horizontalAlign: 'right' }
        };
    }
}
