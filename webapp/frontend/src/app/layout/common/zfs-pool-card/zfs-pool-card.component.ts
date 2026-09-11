import { Component, EventEmitter, Input, Output, inject, ChangeDetectionStrategy } from '@angular/core';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import { Subject } from 'rxjs';

dayjs.extend(relativeTime);
import { MatDialog } from '@angular/material/dialog';
import { ZFSPoolModel, ZFSPoolStatus, zfsUsableSize } from 'app/core/models/zfs-pool-model';
import { AppConfig } from 'app/core/config/app.config';
import { ZFSPoolsService } from 'app/modules/zfs-pools/zfs-pools.service';
import { NgClass, DatePipe } from '@angular/common';
import { MatIcon } from '@angular/material/icon';
import { RouterLink } from '@angular/router';
import { MatIconButton } from '@angular/material/button';
import { MatMenuTrigger, MatMenu, MatMenuItem } from '@angular/material/menu';
import { MenuTriggerRestoreFocusDirective } from 'app/shared/menu-trigger-restore-focus.directive';
import { FileSizePipe } from '../../../shared/file-size.pipe';

@Component({
    selector: 'app-zfs-pool-card',
    templateUrl: './zfs-pool-card.component.html',
    styleUrls: ['./zfs-pool-card.component.scss'],
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [NgClass, MatIcon, RouterLink, MatIconButton, MatMenuTrigger, MenuTriggerRestoreFocusDirective, MatMenu, MatMenuItem, DatePipe, FileSizePipe],
})
export class ZFSPoolCardComponent {
    private readonly _zfsPoolsService = inject(ZFSPoolsService);
    dialog = inject(MatDialog);

    constructor() {
        this._unsubscribeAll = new Subject();
    }

    @Input() poolSummary: ZFSPoolModel;
    @Input() config: AppConfig;
    @Output() poolArchived = new EventEmitter<string>();
    @Output() poolUnarchived = new EventEmitter<string>();
    @Output() poolDeleted = new EventEmitter<string>();

    private readonly _unsubscribeAll: Subject<void>;

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    getPoolStatus(pool: ZFSPoolModel): 'passed' | 'failed' | 'unknown' {
        if (!pool) {
            return 'unknown';
        }
        if (pool.presence && pool.presence !== 'present') {
            return 'unknown';
        }
        switch (pool.status) {
            case 'ONLINE':
                return 'passed';
            case 'DEGRADED':
            case 'FAULTED':
                return 'failed';
            default:
                return 'unknown';
        }
    }

    getStatusColorClass(status: ZFSPoolStatus): string {
        switch (status) {
            case 'ONLINE':
                return 'text-green-600 dark:text-green-400';
            case 'DEGRADED':
                return 'text-yellow-600 dark:text-yellow-400';
            case 'FAULTED':
            case 'UNAVAIL':
            case 'OFFLINE':
            case 'REMOVED':
                return 'text-red-600 dark:text-red-400';
            default:
                return '';
        }
    }

    classPoolLastUpdatedOn(pool: ZFSPoolModel): string {
        if (pool.presence === 'missing' || pool.presence === 'stale') {
            return 'text-red-600 dark:text-red-400';
        } else if (pool.presence === 'unknown') {
            return 'text-yellow-600 dark:text-yellow-400';
        }
        const poolStatus = this.getPoolStatus(pool);
        const lastObservedAt = this.getLastObservedAt(pool);
        if (poolStatus === 'failed') {
            return 'text-red-600 dark:text-red-400';
        } else if (poolStatus === 'passed') {
            if (dayjs().subtract(14, 'day').isBefore(dayjs(lastObservedAt))) {
                return 'text-green-600 dark:text-green-400';
            } else if (dayjs().subtract(1, 'month').isBefore(dayjs(lastObservedAt))) {
                return 'text-yellow-600 dark:text-yellow-400';
            } else {
                return 'text-red-600 dark:text-red-400';
            }
        } else {
            return '';
        }
    }

    getPresenceLabel(pool: ZFSPoolModel): string {
        switch (pool?.presence) {
            case 'missing':
                return 'Missing from last inventory';
            case 'stale':
                return 'Collector stale';
            case 'unknown':
                return 'Inventory unavailable';
            default:
                return '';
        }
    }

    getPresenceColorClass(pool: ZFSPoolModel): string {
        return pool?.presence === 'unknown' ? 'text-yellow-600 dark:text-yellow-400' : 'text-red-600 dark:text-red-400';
    }

    getLastObservedAt(pool: ZFSPoolModel): string {
        if (pool?.last_seen_at && !pool.last_seen_at.startsWith('0001-')) {
            return pool.last_seen_at;
        }
        return pool.updated_at;
    }

    getStatusDisplay(pool: ZFSPoolModel): string {
        switch (pool?.presence) {
            case 'missing':
                return 'MISSING';
            case 'stale':
                return 'STALE';
            case 'unknown':
                return 'UNKNOWN';
            default:
                return pool?.status || 'UNKNOWN';
        }
    }

    getStatusDisplayColorClass(pool: ZFSPoolModel): string {
        if (pool?.presence === 'missing' || pool?.presence === 'stale') {
            return 'text-red-600 dark:text-red-400';
        }
        if (pool?.presence === 'unknown') {
            return 'text-yellow-600 dark:text-yellow-400';
        }
        return pool ? this.getStatusColorClass(pool.status) : '';
    }

    getPoolTitle(pool: ZFSPoolModel): string {
        if (pool.label) {
            return pool.label;
        }
        return pool.name;
    }

    usableSize = zfsUsableSize;

    getCapacityPercentClass(percent: number): string {
        if (percent >= 90) {
            return 'bg-red-500';
        } else if (percent >= 80) {
            return 'bg-yellow-500';
        } else {
            return 'bg-green-500';
        }
    }

    getScrubStatusText(pool: ZFSPoolModel): string {
        switch (pool.scrub_state) {
            case 'none':
                return 'Never';
            case 'scanning':
                return `In Progress (${pool.scrub_percent_complete}%)`;
            case 'finished': {
                const timeAgo = dayjs(pool.scrub_end_time).fromNow();
                if (pool.scrub_issued_bytes > 0) {
                    return `${timeAgo} (repaired)`;
                }
                return timeAgo;
            }
            case 'canceled':
                return 'Canceled';
            default:
                return 'Unknown';
        }
    }

    archivePool(): void {
        if (this.poolSummary.archived) {
            this._zfsPoolsService.unarchivePool(this.poolSummary.guid).subscribe(() => {
                this.poolUnarchived.emit(this.poolSummary.guid);
            });
        } else {
            this._zfsPoolsService.archivePool(this.poolSummary.guid).subscribe(() => {
                this.poolArchived.emit(this.poolSummary.guid);
            });
        }
    }

    deletePool(): void {
        if (confirm(`Are you sure you want to delete pool "${this.getPoolTitle(this.poolSummary)}"?`)) {
            this._zfsPoolsService.deletePool(this.poolSummary.guid).subscribe(() => {
                this.poolDeleted.emit(this.poolSummary.guid);
            });
        }
    }

    getTotalErrors(pool: ZFSPoolModel): number {
        return pool.total_read_errors + pool.total_write_errors + pool.total_checksum_errors;
    }
}
