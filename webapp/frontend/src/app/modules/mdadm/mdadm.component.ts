import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnDestroy, OnInit, ViewEncapsulation, inject } from '@angular/core';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { MDADMService } from 'app/modules/mdadm/mdadm.service';
import { MDADMArrayModel } from 'app/core/models/mdadm-array-model';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { AppConfig } from 'app/core/config/app.config';
import { getMdadmArrayStatusColorClass } from 'app/modules/mdadm/mdadm-status.util';
import { MatIcon } from '@angular/material/icon';
import { MatIconButton } from '@angular/material/button';
import { MatMenu, MatMenuItem, MatMenuTrigger } from '@angular/material/menu';
import { MenuTriggerRestoreFocusDirective } from 'app/shared/menu-trigger-restore-focus.directive';
import { RouterLink } from '@angular/router';
import { NgClass, KeyValuePipe } from '@angular/common';
import { FileSizePipe } from '../../shared/file-size.pipe';

@Component({
    selector: 'mdadm',
    templateUrl: './mdadm.component.html',
    styleUrls: ['./mdadm.component.scss'],
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [MatIcon, MatIconButton, MatMenu, MatMenuItem, MatMenuTrigger, MenuTriggerRestoreFocusDirective, RouterLink, NgClass, FileSizePipe, KeyValuePipe],
})
export class MDADMComponent implements OnInit, OnDestroy {
    private readonly _mdadmService = inject(MDADMService);
    private readonly _configService = inject(ScrutinyConfigService);
    private readonly _changeDetectorRef = inject(ChangeDetectorRef);

    arrays: MDADMArrayModel[] = [];
    hostGroups: { [hostId: string]: string[] } = {};
    config: AppConfig;
    private readonly _unsubscribeAll: Subject<void>;

    constructor() {
        this._unsubscribeAll = new Subject();
    }

    ngOnInit(): void {
        this._configService.config$.pipe(takeUntil(this._unsubscribeAll)).subscribe((config) => {
            this.config = config;
            this._changeDetectorRef.markForCheck();
        });

        this._mdadmService
            .getSummaryData()
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((arrays) => {
                this.arrays = arrays;
                this.hostGroups = {};
                for (const array of arrays) {
                    const hostId = array.host_id || '';
                    const group = this.hostGroups[hostId] || [];
                    group.push(array.uuid);
                    this.hostGroups[hostId] = group;
                }
                this._changeDetectorRef.markForCheck();
            });
    }

    ngOnDestroy(): void {
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    arraysForHostGroup(uuids: string[]): MDADMArrayModel[] {
        return uuids.map((uuid) => this.arrays.find((a) => a.uuid === uuid)).filter(Boolean) as MDADMArrayModel[];
    }

    getArrayStatusColorClass(array: MDADMArrayModel): string {
        return getMdadmArrayStatusColorClass(array.state);
    }

    trackByFn(index: number, item: any): any {
        return item.uuid || index;
    }

    toggleArchived(array: MDADMArrayModel): void {
        const request = array.archived ? this._mdadmService.unarchiveArray(array.uuid) : this._mdadmService.archiveArray(array.uuid);
        request.pipe(takeUntil(this._unsubscribeAll)).subscribe(() => {
            array.archived = !array.archived;
            this._changeDetectorRef.markForCheck();
        });
    }

    toggleMuted(array: MDADMArrayModel): void {
        const request = array.muted ? this._mdadmService.unmuteArray(array.uuid) : this._mdadmService.muteArray(array.uuid);
        request.pipe(takeUntil(this._unsubscribeAll)).subscribe(() => {
            array.muted = !array.muted;
            this._changeDetectorRef.markForCheck();
        });
    }

    // Deleting drops the array's metrics history with its metadata. An array
    // the collector still reports is registered again on its next run.
    deleteArray(array: MDADMArrayModel): void {
        if (!confirm(`Are you sure you want to delete array "${array.label || array.name}"? Its metrics history is deleted with it.`)) {
            return;
        }
        this._mdadmService
            .deleteArray(array.uuid)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(() => {
                this.arrays = this.arrays.filter((candidate) => candidate.uuid !== array.uuid);
                for (const hostId of Object.keys(this.hostGroups)) {
                    this.hostGroups[hostId] = this.hostGroups[hostId].filter((uuid) => uuid !== array.uuid);
                    if (this.hostGroups[hostId].length === 0) {
                        delete this.hostGroups[hostId];
                    }
                }
                this._changeDetectorRef.markForCheck();
            });
    }
}
