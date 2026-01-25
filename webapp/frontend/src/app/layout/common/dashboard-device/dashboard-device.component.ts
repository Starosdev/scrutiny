import {Component, EventEmitter, Input, OnInit, Output} from '@angular/core';
import dayjs from 'dayjs';
import {takeUntil} from 'rxjs/operators';
import {AppConfig} from 'app/core/config/app.config';
import {ScrutinyConfigService} from 'app/core/config/scrutiny-config.service';
import {Subject} from 'rxjs';
import {MatDialog as MatDialog} from '@angular/material/dialog';
import {DashboardDeviceDeleteDialogComponent} from 'app/layout/common/dashboard-device-delete-dialog/dashboard-device-delete-dialog.component';
import {DeviceTitlePipe} from 'app/shared/device-title.pipe';
import {DeviceSummaryModel} from 'app/core/models/device-summary-model';
import {DeviceStatusPipe} from 'app/shared/device-status.pipe';
import {DashboardDeviceArchiveDialogComponent} from '../dashboard-device-archive-dialog/dashboard-device-archive-dialog.component';
import {DashboardDeviceArchiveDialogService} from '../dashboard-device-archive-dialog/dashboard-device-archive-dialog.service';

@Component({
    selector: 'app-dashboard-device',
    templateUrl: './dashboard-device.component.html',
    styleUrls: ['./dashboard-device.component.scss'],
    standalone: false
})
export class DashboardDeviceComponent implements OnInit {

    constructor(
        private _configService: ScrutinyConfigService,
        private _archiveService: DashboardDeviceArchiveDialogService,
        public dialog: MatDialog,
    ) {
        // Set the private defaults
        this._unsubscribeAll = new Subject();
    }

    @Input() deviceSummary: DeviceSummaryModel;
    @Output() deviceArchived = new EventEmitter<string>();
    @Output() deviceUnarchived = new EventEmitter<string>();
    @Output() deviceDeleted = new EventEmitter<string>();

    config: AppConfig;

    private _unsubscribeAll: Subject<void>;

    deviceStatusForModelWithThreshold = DeviceStatusPipe.deviceStatusForModelWithThreshold

    ngOnInit(): void {
        // Subscribe to config changes
        this._configService.config$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((config: AppConfig) => {
                this.config = config;
            });
    }


    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    classDeviceLastUpdatedOn(deviceSummary: DeviceSummaryModel): string {
        const deviceStatus = DeviceStatusPipe.deviceStatusForModelWithThreshold(deviceSummary.device, !!deviceSummary.smart, this.config.metrics.status_threshold)
        if (deviceStatus === 'failed') {
            return 'text-red' // if the device has failed, always highlight in red
        } else if (deviceStatus === 'passed') {
            if (dayjs().subtract(14, 'day').isBefore(dayjs(deviceSummary.smart.collector_date))) {
                // this device was updated in the last 2 weeks.
                return 'text-green'
            } else if (dayjs().subtract(1, 'month').isBefore(dayjs(deviceSummary.smart.collector_date))) {
                // this device was updated in the last month
                return 'text-yellow'
            } else {
                // last updated more than a month ago.
                return 'text-red'
            }
        } else {
            return ''
        }
    }

    openArchiveDialog(): void {
        if(this.deviceSummary.device.archived){
            this._archiveService.unarchiveDevice(this.deviceSummary.device.wwn).subscribe((result) => {
                if(result) {
                    this.deviceUnarchived.emit(this.deviceSummary.device.wwn)
                }
            })
            return;
        }
        const dialogRef = this.dialog.open(DashboardDeviceArchiveDialogComponent, {
            data: {
                wwn: this.deviceSummary.device.wwn,
                title: DeviceTitlePipe.deviceTitleWithFallback(this.deviceSummary.device, this.config.dashboard_display)
            }
        });
        dialogRef.afterClosed().subscribe(result => {
            if(result) {
                this.deviceArchived.emit(this.deviceSummary.device.wwn);
            }
        })
    }

    openDeleteDialog(): void {
        const dialogRef = this.dialog.open(DashboardDeviceDeleteDialogComponent, {
            // width: '250px',
            data: {
                wwn: this.deviceSummary.device.wwn,
                title: DeviceTitlePipe.deviceTitleWithFallback(this.deviceSummary.device, this.config.dashboard_display)
            }
        });

        dialogRef.afterClosed().subscribe(result => {
            if (result?.success) {
                this.deviceDeleted.emit(this.deviceSummary.device.wwn)
            }
        });
    }

    /**
     * Get SSD health value for display.
     * Returns an object with the value (0-100) and whether it represents "remaining" health
     * (where higher is better) or "used" percentage (where lower is better).
     *
     * - percentage_used (NVMe/ATA devstat): 0-100%, higher = more worn
     * - wearout_value (ATA 177/233/231/232): 0-100%, higher = healthier
     */
    getSSDHealth(): { value: number; isRemaining: boolean } | null {
        if (!this.deviceSummary?.smart) {
            return null;
        }

        // Priority: percentage_used first (more direct metric), then wearout_value
        if (this.deviceSummary.smart.percentage_used != null) {
            return {
                value: this.deviceSummary.smart.percentage_used,
                isRemaining: false // percentage_used: higher = more worn
            };
        }

        if (this.deviceSummary.smart.wearout_value != null) {
            return {
                value: this.deviceSummary.smart.wearout_value,
                isRemaining: true // wearout_value: higher = healthier
            };
        }

        return null;
    }

    /**
     * Get display label for SSD health metric
     */
    getSSDHealthLabel(): string {
        const health = this.getSSDHealth();
        if (!health) {
            return '';
        }
        return health.isRemaining ? 'Health' : 'Used';
    }

    /**
     * Get formatted display value for SSD health
     * For "remaining" health, shows as-is (e.g., "95%")
     * For "used" percentage, shows as-is (e.g., "5%")
     */
    getSSDHealthDisplay(): string {
        const health = this.getSSDHealth();
        if (!health) {
            return '--';
        }
        return `${health.value}%`;
    }
}
