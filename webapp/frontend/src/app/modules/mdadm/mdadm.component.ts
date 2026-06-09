import { ChangeDetectionStrategy, ChangeDetectorRef, Component, OnDestroy, OnInit, ViewEncapsulation, inject } from '@angular/core';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { MDADMService } from 'app/modules/mdadm/mdadm.service';
import { MDADMArrayModel } from 'app/core/models/mdadm-array-model';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { AppConfig } from 'app/core/config/app.config';
import { getMdadmArrayStatusColorClass } from 'app/modules/mdadm/mdadm-status.util';
import { MatIcon } from '@angular/material/icon';
import { RouterLink } from '@angular/router';
import { NgClass, KeyValuePipe } from '@angular/common';
import { FileSizePipe } from '../../shared/file-size.pipe';

@Component({
    selector: 'mdadm',
    templateUrl: './mdadm.component.html',
    styleUrls: ['./mdadm.component.scss'],
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [MatIcon, RouterLink, NgClass, FileSizePipe, KeyValuePipe],
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
}
