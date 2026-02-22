import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnDestroy, OnInit, ViewEncapsulation } from '@angular/core';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { WorkloadService } from 'app/modules/workload/workload.service';
import { WorkloadInsightModel } from 'app/core/models/workload-insight-model';
import { AppConfig } from 'app/core/config/app.config';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { Router } from '@angular/router';
import { MatSort, Sort } from '@angular/material/sort';
import { MatTableDataSource } from '@angular/material/table';
import { ViewChild, AfterViewInit } from '@angular/core';

@Component({
    selector: 'workload',
    templateUrl: './workload.component.html',
    styleUrls: ['./workload.component.scss'],
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
})
export class WorkloadComponent implements OnInit, AfterViewInit, OnDestroy {
    workloadData: Record<string, WorkloadInsightModel>;
    config: AppConfig;
    durationKey = 'week';
    dataSource: MatTableDataSource<WorkloadInsightModel>;
    displayedColumns: string[] = ['device_wwn', 'device_protocol', 'daily_writes', 'daily_reads', 'rw_ratio', 'intensity', 'endurance', 'est_remaining'];
    spikeDevices: WorkloadInsightModel[] = [];

    @ViewChild(MatSort) sort: MatSort;
    private _unsubscribeAll: Subject<void>;

    constructor(
        private _workloadService: WorkloadService,
        private _configService: ScrutinyConfigService,
        private _changeDetectorRef: ChangeDetectorRef,
        private router: Router
    ) {
        this._unsubscribeAll = new Subject();
        this.dataSource = new MatTableDataSource([]);
    }

    ngOnInit(): void {
        this._configService.config$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((config: AppConfig) => {
                const oldConfig = JSON.stringify(this.config);
                const newConfig = JSON.stringify(config);

                if (oldConfig !== newConfig) {
                    this.config = config;
                    if (oldConfig) {
                        this.refreshComponent();
                    }
                }
            });

        this._workloadService.data$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((data) => {
                this.workloadData = data;
                if (data) {
                    const insights = Object.values(data);
                    this.dataSource.data = insights;
                    this.spikeDevices = insights.filter(d => d.spike?.detected);
                    this._changeDetectorRef.markForCheck();
                }
            });
    }

    ngAfterViewInit(): void {
        this.dataSource.sort = this.sort;
        this.dataSource.sortingDataAccessor = (item: WorkloadInsightModel, property: string) => {
            switch (property) {
                case 'daily_writes': return item.daily_write_bytes;
                case 'daily_reads': return item.daily_read_bytes;
                case 'rw_ratio': return item.read_write_ratio;
                case 'endurance': return item.endurance?.percentage_used ?? -1;
                case 'est_remaining': return item.endurance?.estimated_lifespan_days ?? -1;
                default: return item[property];
            }
        };
    }

    ngOnDestroy(): void {
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    changeDuration(durationKey: string): void {
        this.durationKey = durationKey;
        this._workloadService.getWorkloadData(durationKey).subscribe();
    }

    navigateToDevice(wwn: string): void {
        this.router.navigate(['/device', wwn]);
    }

    intensityColor(intensity: string): string {
        switch (intensity) {
            case 'idle': return 'text-blue-400';
            case 'light': return 'text-green-500';
            case 'medium': return 'text-yellow-500';
            case 'heavy': return 'text-red-500';
            default: return 'text-gray-400';
        }
    }

    enduranceColor(percentageUsed: number): string {
        if (percentageUsed >= 90) return 'warn';
        if (percentageUsed >= 70) return 'accent';
        return 'primary';
    }

    formatDays(days: number): string {
        if (!days || days <= 0) return '-';
        if (days >= 365) {
            const years = (days / 365).toFixed(1);
            return `${years} yr`;
        }
        return `${days} d`;
    }

    private refreshComponent(): void {
        const currentUrl = this.router.url;
        this.router.routeReuseStrategy.shouldReuseRoute = () => false;
        this.router.onSameUrlNavigation = 'reload';
        this.router.navigate([currentUrl]);
    }
}
