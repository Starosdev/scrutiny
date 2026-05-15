import { ChangeDetectionStrategy, Component, OnDestroy, OnInit, ViewEncapsulation } from '@angular/core';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { AppConfig } from 'app/core/config/app.config';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { Router } from '@angular/router';
import { BtrfsFilesystemModel } from 'app/core/models/btrfs-filesystem-model';
import { BtrfsFilesystemsService } from 'app/modules/btrfs-filesystems/btrfs-filesystems.service';

@Component({
    selector: 'btrfs-filesystems',
    templateUrl: './btrfs-filesystems.component.html',
    styleUrls: ['./btrfs-filesystems.component.scss'],
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
})
export class BtrfsFilesystemsComponent implements OnInit, OnDestroy {
    summaryData: Record<string, BtrfsFilesystemModel>;
    hostGroups: { [hostId: string]: string[] } = {};
    config: AppConfig;
    showArchived: boolean;

    private _unsubscribeAll: Subject<void> = new Subject();

    constructor(private readonly _btrfsFilesystemsService: BtrfsFilesystemsService, private readonly _configService: ScrutinyConfigService, private readonly router: Router) {}

    ngOnInit(): void {
        this._configService.config$.pipe(takeUntil(this._unsubscribeAll)).subscribe((config: AppConfig) => {
            this.config = config;
        });

        this._btrfsFilesystemsService.data$.pipe(takeUntil(this._unsubscribeAll)).subscribe((data) => {
            this.summaryData = data;
            this.hostGroups = {};
            for (const uuid in this.summaryData) {
                const hostId = this.summaryData[uuid].host_id || 'unknown';
                const hostFilesystemList = this.hostGroups[hostId] || [];
                hostFilesystemList.push(uuid);
                this.hostGroups[hostId] = hostFilesystemList;
            }
        });
    }

    ngOnDestroy(): void {
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    filesystemsForHostGroup(hostGroupUUIDs: string[]): BtrfsFilesystemModel[] {
        return hostGroupUUIDs.map((uuid) => this.summaryData[uuid]).filter(Boolean);
    }

    onFilesystemDeleted(uuid: string): void {
        delete this.summaryData[uuid];
    }

    onFilesystemArchived(uuid: string): void {
        this.summaryData[uuid].archived = true;
    }

    onFilesystemUnarchived(uuid: string): void {
        this.summaryData[uuid].archived = false;
    }
}
