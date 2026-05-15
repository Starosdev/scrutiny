import {
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    OnDestroy,
    OnInit,
    ViewEncapsulation
} from '@angular/core';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { MDADMService } from 'app/modules/mdadm/mdadm.service';
import { MDADMArrayModel } from 'app/core/models/mdadm-array-model';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { AppConfig } from 'app/core/config/app.config';
import { getMdadmArrayStatusColorClass } from 'app/modules/mdadm/mdadm-status.util';

@Component({
    selector: 'mdadm',
    templateUrl: './mdadm.component.html',
    styleUrls: ['./mdadm.component.scss'],
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class MDADMComponent implements OnInit, OnDestroy {
    arrays: MDADMArrayModel[] = [];
    config: AppConfig;
    private _unsubscribeAll: Subject<void>;

    constructor(
        private readonly _mdadmService: MDADMService,
        private readonly _configService: ScrutinyConfigService,
        private readonly _changeDetectorRef: ChangeDetectorRef
    ) {
        this._unsubscribeAll = new Subject();
    }

    ngOnInit(): void {
        this._configService.config$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((config) => {
                this.config = config;
                this._changeDetectorRef.markForCheck();
            });

        this._mdadmService.getSummaryData()
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((arrays) => {
                this.arrays = arrays;
                this._changeDetectorRef.markForCheck();
            });
    }

    ngOnDestroy(): void {
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    getArrayStatusColorClass(array: MDADMArrayModel): string {
        return getMdadmArrayStatusColorClass(array.state);
    }

    trackByFn(index: number, item: any): any {
        return item.uuid || index;
    }
}
