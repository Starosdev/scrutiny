import { Component, OnDestroy, ViewEncapsulation } from '@angular/core';
import { Subject } from 'rxjs';

@Component({
    selector: 'empty-layout',
    templateUrl: './empty.component.html',
    styleUrls: ['./empty.component.scss'],
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class EmptyLayoutComponent implements OnDestroy
{
    // Private
    private _unsubscribeAll: Subject<void>;

    /**
     * Constructor
     */
    constructor()
    {
        // Set the private defaults
        this._unsubscribeAll = new Subject();
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Lifecycle hooks
    // -----------------------------------------------------------------------------------------------------

    /**
     * On destroy
     */
    ngOnDestroy(): void
    {
        // Unsubscribe from all subscriptions
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }
}
