import { Component, EventEmitter, Input, OnInit, Output, inject, ChangeDetectionStrategy } from '@angular/core';
import dayjs from 'dayjs';
import { takeUntil } from 'rxjs/operators';
import { AppConfig } from 'app/core/config/app.config';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { Subject } from 'rxjs';
import { MatDialog as MatDialog } from '@angular/material/dialog';
import { DashboardDeviceDeleteDialogComponent } from 'app/layout/common/dashboard-device-delete-dialog/dashboard-device-delete-dialog.component';
import { DeviceTitlePipe } from 'app/shared/device-title.pipe';
import { DeviceSummaryModel } from 'app/core/models/device-summary-model';
import { FilesystemCapacityModel } from 'app/core/models/filesystem-summary-model';
import { DeviceStatusPipe } from 'app/shared/device-status.pipe';
import { DashboardDeviceArchiveDialogComponent } from '../dashboard-device-archive-dialog/dashboard-device-archive-dialog.component';
import { DashboardDeviceArchiveDialogService } from '../dashboard-device-archive-dialog/dashboard-device-archive-dialog.service';
import { NgClass, TitleCasePipe, DatePipe } from '@angular/common';
import { MatIcon } from '@angular/material/icon';
import { RouterLink } from '@angular/router';
import { MatIconButton } from '@angular/material/button';
import { MatMenuTrigger, MatMenu, MatMenuItem } from '@angular/material/menu';
import { MatTooltip } from '@angular/material/tooltip';
import { MenuTriggerRestoreFocusDirective } from 'app/shared/menu-trigger-restore-focus.directive';
import { FileSizePipe } from '../../../shared/file-size.pipe';
import { TemperaturePipe } from '../../../shared/temperature.pipe';
import { DeviceHoursPipe } from '../../../shared/device-hours.pipe';

@Component({
    selector: 'app-dashboard-device',
    templateUrl: './dashboard-device.component.html',
    styleUrls: ['./dashboard-device.component.scss'],
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [
        NgClass,
        MatIcon,
        RouterLink,
        MatIconButton,
        MatMenuTrigger,
        MenuTriggerRestoreFocusDirective,
        MatMenu,
        MatMenuItem,
        MatTooltip,
        TitleCasePipe,
        DatePipe,
        FileSizePipe,
        TemperaturePipe,
        DeviceHoursPipe,
    ],
})
export class DashboardDeviceComponent implements OnInit {
    private readonly _configService = inject(ScrutinyConfigService);
    private readonly _archiveService = inject(DashboardDeviceArchiveDialogService);
    dialog = inject(MatDialog);

    constructor() {
        // Set the private defaults
        this._unsubscribeAll = new Subject();
    }

    @Input() deviceSummary: DeviceSummaryModel;
    @Input() compact: boolean = false;
    @Output() deviceArchived = new EventEmitter<string>();
    @Output() deviceUnarchived = new EventEmitter<string>();
    @Output() deviceDeleted = new EventEmitter<string>();

    config: AppConfig;

    private readonly _unsubscribeAll: Subject<void>;

    deviceStatusForModelWithThreshold = DeviceStatusPipe.deviceStatusForModelWithThreshold;

    ngOnInit(): void {
        // Subscribe to config changes
        this._configService.config$.pipe(takeUntil(this._unsubscribeAll)).subscribe((config: AppConfig) => {
            this.config = config;
        });
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    classDeviceLastUpdatedOn(deviceSummary: DeviceSummaryModel): string {
        const deviceStatus = DeviceStatusPipe.deviceStatusForModelWithThreshold(deviceSummary.device, !!deviceSummary.smart, this.config.metrics.status_threshold);
        if (deviceStatus === 'failed') {
            return 'text-red'; // if the device has failed, always highlight in red
        } else if (deviceStatus === 'passed') {
            if (dayjs().subtract(14, 'day').isBefore(dayjs(deviceSummary.smart.collector_date))) {
                // this device was updated in the last 2 weeks.
                return 'text-green';
            } else if (dayjs().subtract(1, 'month').isBefore(dayjs(deviceSummary.smart.collector_date))) {
                // this device was updated in the last month
                return 'text-yellow';
            } else {
                // last updated more than a month ago.
                return 'text-red';
            }
        } else {
            return '';
        }
    }

    deviceDashboardTitle(): string {
        return DeviceTitlePipe.deviceDashboardTitle(this.deviceSummary.device);
    }

    openArchiveDialog(): void {
        if (this.deviceSummary.device.archived) {
            this._archiveService.unarchiveDevice(this.deviceSummary.device.device_id).subscribe((result) => {
                if (result) {
                    this.deviceUnarchived.emit(this.deviceSummary.device.device_id);
                }
            });
            return;
        }
        const dialogRef = this.dialog.open(DashboardDeviceArchiveDialogComponent, {
            data: {
                deviceId: this.deviceSummary.device.device_id,
                title: DeviceTitlePipe.deviceTitleWithFallback(this.deviceSummary.device, this.config.dashboard_display),
            },
        });
        dialogRef.afterClosed().subscribe((result) => {
            if (result) {
                this.deviceArchived.emit(this.deviceSummary.device.device_id);
            }
        });
    }

    openDeleteDialog(): void {
        const dialogRef = this.dialog.open(DashboardDeviceDeleteDialogComponent, {
            // width: '250px',
            data: {
                deviceId: this.deviceSummary.device.device_id,
                title: DeviceTitlePipe.deviceTitleWithFallback(this.deviceSummary.device, this.config.dashboard_display),
            },
        });

        dialogRef.afterClosed().subscribe((result) => {
            if (result?.success) {
                this.deviceDeleted.emit(this.deviceSummary.device.device_id);
            }
        });
    }

    /**
     * Returns a human-readable label for a risk category.
     */
    riskCategoryLabel(category: string): string {
        switch (category) {
            case 'monitor':
                return 'Monitor';
            case 'plan_replacement':
                return 'Plan Replacement';
            case 'replace_soon':
                return 'Replace Soon';
            default:
                return '';
        }
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
                isRemaining: false, // percentage_used: higher = more worn
            };
        }

        if (this.deviceSummary.smart.wearout_value != null) {
            return {
                value: this.deviceSummary.smart.wearout_value,
                isRemaining: true, // wearout_value: higher = healthier
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

    /**
     * The one filesystem whose free space belongs entirely to this drive, or
     * null when there is no such filesystem.
     *
     * A single free-space figure is only honest in that case. A drive holding
     * several filesystems has several free figures, and a filesystem spanning
     * several drives (LVM, md) has free space that belongs to no single drive;
     * dividing it up would invent a number the kernel never reported. Both of
     * those are shown as a mount count here and broken down on the detail page.
     */
    exclusiveFilesystem(): FilesystemCapacityModel | null {
        const filesystems = this.deviceSummary?.filesystems;
        if (filesystems?.length !== 1) {
            return null;
        }
        return filesystems[0].backing_devices?.length === 1 ? filesystems[0] : null;
    }

    filesystemMountCount(): number {
        return this.deviceSummary?.filesystems?.length ?? 0;
    }
}
