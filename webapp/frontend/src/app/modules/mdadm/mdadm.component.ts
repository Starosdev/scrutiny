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
        const state = (array.state || '').toLowerCase();
        if (state.includes('degraded') || state.includes('inactive')) {
            return 'text-red-600 dark:text-red-400 bg-red-100 dark:bg-red-900';
        }
        if (state.includes('checking') || state.includes('resync') || state.includes('recover') || state.includes('rebuild')) {
            return 'text-blue-600 dark:text-blue-400 bg-blue-100 dark:bg-blue-900';
        }
        if (state.includes('clean') || state.includes('active')) {
            return 'text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900';
        }
        return 'text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-800';
    }

    trackByFn(index: number, item: any): any {
        return item.uuid || index;
    }
}
