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
    private _unsubscribeAll: Subject<void>;

    constructor(
        private readonly _mdadmService: MDADMService,
        private readonly _changeDetectorRef: ChangeDetectorRef
    ) {
        this._unsubscribeAll = new Subject();
    }

    ngOnInit(): void {
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
        // This is a placeholder logic until we have status in the model
        // For now, we'll assume clean/active is good.
        return 'text-green-600 dark:text-green-400';
    }

    trackByFn(index: number, item: any): any {
        return item.uuid || index;
    }
}
