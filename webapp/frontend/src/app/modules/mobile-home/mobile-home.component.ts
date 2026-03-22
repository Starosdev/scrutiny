import { Component, OnDestroy, OnInit, ViewEncapsulation } from '@angular/core';
import { Router } from '@angular/router';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { DashboardService } from 'app/modules/dashboard/dashboard.service';
import { ZFSPoolsService } from 'app/modules/zfs-pools/zfs-pools.service';
import { DeviceSummaryModel } from 'app/core/models/device-summary-model';
import { ZFSPoolModel } from 'app/core/models/zfs-pool-model';

interface HealthCounts {
    healthy: number;
    warning: number;
    critical: number;
}

interface AttentionItem {
    type: 'drive' | 'zfs';
    id: string;
    name: string;
    host: string;
    status: string;
    temperature?: number;
    route: string;
}

@Component({
    selector: 'mobile-home',
    templateUrl: './mobile-home.component.html',
    styleUrls: ['./mobile-home.component.scss'],
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class MobileHomeComponent implements OnInit, OnDestroy {
    driveCounts: HealthCounts = { healthy: 0, warning: 0, critical: 0 };
    zfsCounts: HealthCounts = { healthy: 0, warning: 0, critical: 0 };
    attentionItems: AttentionItem[] = [];
    loaded: boolean = false;

    private _unsubscribeAll: Subject<void>;

    constructor(
        private readonly _dashboardService: DashboardService,
        private readonly _zfsPoolsService: ZFSPoolsService,
        private readonly _router: Router
    ) {
        this._unsubscribeAll = new Subject();
    }

    ngOnInit(): void {
        this._dashboardService.data$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(data => {
                if (data) {
                    this._processDriveData(data);
                    this.loaded = true;
                }
            });

        this._zfsPoolsService.data$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(data => {
                if (data) {
                    this._processZFSData(data);
                }
            });
    }

    ngOnDestroy(): void {
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    navigateTo(item: AttentionItem): void {
        this._router.navigate([item.route]);
    }

    private _processDriveData(data: { [key: string]: DeviceSummaryModel }): void {
        this.driveCounts = { healthy: 0, warning: 0, critical: 0 };
        const driveAttention: AttentionItem[] = [];

        for (const wwn in data) {
            const device = data[wwn];
            if (device.device.archived) {
                continue;
            }

            // device_status: 0 = passed, non-zero = problem
            const status = device.device.device_status;
            if (!status || status === 0) {
                this.driveCounts.healthy++;
            } else {
                // Use status >= 3 as critical (failed), others as warning
                const severity = status >= 3 ? 'critical' : 'warning';
                if (severity === 'critical') {
                    this.driveCounts.critical++;
                } else {
                    this.driveCounts.warning++;
                }
                driveAttention.push({
                    type: 'drive',
                    id: wwn,
                    name: device.device.device_name || device.device.model_name || wwn,
                    host: device.device.host_id || '',
                    status: severity,
                    temperature: device.smart?.temp,
                    route: `/device/${wwn}`
                });
            }
        }

        // Merge drive attention items (critical first)
        this.attentionItems = [
            ...driveAttention.filter(i => i.status === 'critical'),
            ...driveAttention.filter(i => i.status === 'warning'),
            ...this.attentionItems.filter(i => i.type === 'zfs')
        ];
    }

    private _processZFSData(data: Record<string, ZFSPoolModel>): void {
        this.zfsCounts = { healthy: 0, warning: 0, critical: 0 };
        const zfsAttention: AttentionItem[] = [];

        for (const guid in data) {
            const pool = data[guid];
            if (pool.archived) {
                continue;
            }

            // status: 'ONLINE' = healthy, 'DEGRADED' = warning, others = critical
            if (pool.status === 'ONLINE') {
                this.zfsCounts.healthy++;
            } else if (pool.status === 'DEGRADED') {
                this.zfsCounts.warning++;
                zfsAttention.push({
                    type: 'zfs',
                    id: guid,
                    name: pool.name || guid,
                    host: pool.host_id || '',
                    status: 'warning',
                    route: `/zfs-pool/${guid}`
                });
            } else {
                this.zfsCounts.critical++;
                zfsAttention.push({
                    type: 'zfs',
                    id: guid,
                    name: pool.name || guid,
                    host: pool.host_id || '',
                    status: 'critical',
                    route: `/zfs-pool/${guid}`
                });
            }
        }

        // Merge ZFS attention items
        this.attentionItems = [
            ...this.attentionItems.filter(i => i.type === 'drive'),
            ...zfsAttention.filter(i => i.status === 'critical'),
            ...zfsAttention.filter(i => i.status === 'warning')
        ];
    }
}
