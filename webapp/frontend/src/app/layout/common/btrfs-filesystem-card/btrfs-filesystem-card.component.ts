import { Component, EventEmitter, Input, Output, inject } from '@angular/core';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import { MatDialog } from '@angular/material/dialog';
import { AppConfig } from 'app/core/config/app.config';
import { BtrfsFilesystemModel, BtrfsFilesystemStatus } from 'app/core/models/btrfs-filesystem-model';
import { BtrfsFilesystemsService } from 'app/modules/btrfs-filesystems/btrfs-filesystems.service';

dayjs.extend(relativeTime);

@Component({
    selector: 'app-btrfs-filesystem-card',
    templateUrl: './btrfs-filesystem-card.component.html',
    styleUrls: ['./btrfs-filesystem-card.component.scss'],
    standalone: false,
})
export class BtrfsFilesystemCardComponent {
    private readonly _btrfsFilesystemsService = inject(BtrfsFilesystemsService);
    dialog = inject(MatDialog);

    @Input() filesystemSummary: BtrfsFilesystemModel;
    @Input() config: AppConfig;
    @Output() filesystemArchived = new EventEmitter<string>();
    @Output() filesystemUnarchived = new EventEmitter<string>();
    @Output() filesystemDeleted = new EventEmitter<string>();

    getStatusColorClass(status: BtrfsFilesystemStatus): string {
        return status === 'ONLINE' ? 'text-green-600 dark:text-green-400' : 'text-yellow-600 dark:text-yellow-400';
    }

    getFilesystemTitle(filesystem: BtrfsFilesystemModel): string {
        return filesystem.label || filesystem.mount_point || filesystem.uuid;
    }

    classFilesystemLastUpdatedOn(filesystem: BtrfsFilesystemModel): string {
        if (dayjs().subtract(14, 'day').isBefore(dayjs(filesystem.updated_at))) {
            return filesystem.status === 'ONLINE' ? 'text-green-600 dark:text-green-400' : 'text-yellow-600 dark:text-yellow-400';
        }
        return 'text-red-600 dark:text-red-400';
    }

    getUsagePercent(filesystem: BtrfsFilesystemModel): number {
        if (filesystem.device_size <= 0) {
            return 0;
        }
        return Number(((filesystem.used / filesystem.device_size) * 100).toFixed(1));
    }

    getUsageClass(percent: number): string {
        if (percent >= 90) {
            return 'bg-red-500';
        }
        if (percent >= 80) {
            return 'bg-yellow-500';
        }
        return 'bg-green-500';
    }

    getErrorCount(filesystem: BtrfsFilesystemModel): number {
        return filesystem.scrub_read_errors + filesystem.scrub_csum_errors + filesystem.scrub_verify_errors + filesystem.scrub_super_errors;
    }

    archiveFilesystem(): void {
        if (this.filesystemSummary.archived) {
            this._btrfsFilesystemsService.unarchiveFilesystem(this.filesystemSummary.uuid).subscribe(() => this.filesystemUnarchived.emit(this.filesystemSummary.uuid));
        } else {
            this._btrfsFilesystemsService.archiveFilesystem(this.filesystemSummary.uuid).subscribe(() => this.filesystemArchived.emit(this.filesystemSummary.uuid));
        }
    }

    deleteFilesystem(): void {
        if (confirm(`Are you sure you want to delete filesystem "${this.getFilesystemTitle(this.filesystemSummary)}"?`)) {
            this._btrfsFilesystemsService.deleteFilesystem(this.filesystemSummary.uuid).subscribe(() => this.filesystemDeleted.emit(this.filesystemSummary.uuid));
        }
    }
}
