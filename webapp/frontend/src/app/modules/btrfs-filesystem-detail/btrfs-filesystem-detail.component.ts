import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnDestroy, OnInit, ViewEncapsulation } from '@angular/core';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { ApexOptions } from 'ng-apexcharts';
import { AppConfig } from 'app/core/config/app.config';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { Router } from '@angular/router';
import { apexShortDateTime } from 'app/shared/time-format.utils';
import { BtrfsFilesystemModel, BtrfsDeviceModel } from 'app/core/models/btrfs-filesystem-model';
import { BtrfsMetricsHistoryModel } from 'app/core/models/btrfs-filesystem-summary-model';
import { BtrfsFilesystemDetailService } from 'app/modules/btrfs-filesystem-detail/btrfs-filesystem-detail.service';

@Component({
    selector: 'btrfs-filesystem-detail',
    templateUrl: './btrfs-filesystem-detail.component.html',
    styleUrls: ['./btrfs-filesystem-detail.component.scss'],
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
})
export class BtrfsFilesystemDetailComponent implements OnInit, OnDestroy {
    filesystem: BtrfsFilesystemModel;
    metricsHistory: BtrfsMetricsHistoryModel[];
    usageOptions: ApexOptions;
    config: AppConfig;
    private _unsubscribeAll: Subject<void> = new Subject();

    constructor(
        private readonly _btrfsFilesystemDetailService: BtrfsFilesystemDetailService,
        private readonly _configService: ScrutinyConfigService,
        private readonly _changeDetectorRef: ChangeDetectorRef,
        private readonly router: Router
    ) {}

    ngOnInit(): void {
        this._configService.config$.pipe(takeUntil(this._unsubscribeAll)).subscribe((config: AppConfig) => {
            this.config = config;
            this._changeDetectorRef.markForCheck();
        });

        this._btrfsFilesystemDetailService.data$.pipe(takeUntil(this._unsubscribeAll)).subscribe((data) => {
            if (data) {
                this.filesystem = data.data.filesystem;
                this.metricsHistory = data.data.metrics_history;
                this._prepareChartData();
                this._changeDetectorRef.markForCheck();
            }
        });
    }

    ngOnDestroy(): void {
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    private _prepareChartData(): void {
        if (!this.metricsHistory || this.metricsHistory.length === 0) {
            return;
        }
        const usageData = this.metricsHistory.map((m) => ({
            x: new Date(m.date),
            y: m.device_size > 0 ? Number(((m.used / m.device_size) * 100).toFixed(1)) : 0,
        }));
        this.usageOptions = {
            chart: {
                animations: { speed: 400, animateGradually: { enabled: false } },
                fontFamily: 'inherit',
                foreColor: 'inherit',
                width: '100%',
                height: '100%',
                type: 'area',
                sparkline: { enabled: true },
            },
            colors: ['#22c55e'],
            fill: { colors: ['#bbf7d0'], opacity: 0.5, type: 'gradient' },
            series: [{ name: 'Usage', data: usageData }],
            stroke: { curve: 'smooth', width: 2 },
            tooltip: { theme: 'dark', x: { format: apexShortDateTime(this.config.time_format, true) }, y: { formatter: (value) => `${value}%` } },
            xaxis: { type: 'datetime', labels: { datetimeUTC: false } },
            yaxis: { min: 0, max: 100 },
        };
    }

    getFilesystemTitle(): string {
        return this.filesystem?.label || this.filesystem?.mount_point || this.filesystem?.uuid || 'Unknown Filesystem';
    }

    getErrorCount(device: BtrfsDeviceModel): number {
        return device.read_io_errors + device.write_io_errors + device.flush_io_errors + device.corruption_errors + device.generation_errors;
    }

    toggleMuted(): void {
        const newMutedState = !this.filesystem.muted;
        this._btrfsFilesystemDetailService.setMuted(this.filesystem.uuid, newMutedState).subscribe(() => {
            this.filesystem.muted = newMutedState;
        });
    }

    goBack(): void {
        this.router.navigate(['/btrfs-filesystems']);
    }
}
