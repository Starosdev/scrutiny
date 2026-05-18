import { Injectable, DOCUMENT, inject } from '@angular/core';

import { NavigationEnd, Router } from '@angular/router';
import { filter, take } from 'rxjs/operators';

@Injectable()
export class TreoSplashScreenService {
    private _document = inject(DOCUMENT);
    private _router = inject(Router);

    /**
     * Constructor
     *
     * @param {DOCUMENT} _document
     * @param {Router} _router
     */
    constructor() {
        // Initialize
        this._init();
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Private methods
    // -----------------------------------------------------------------------------------------------------

    /**
     * Initialize
     *
     * @private
     */
    private _init(): void {
        // Hide it on the first NavigationEnd event
        this._router.events
            .pipe(
                filter((event) => event instanceof NavigationEnd),
                take(1)
            )
            .subscribe(() => {
                // Hide the splash screen
                this.hide();
            });
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    /**
     * Show the splash screen
     */
    show(): void {
        this._document.body.classList.remove('treo-splash-screen-hidden');
    }

    /**
     * Hide the splash screen
     */
    hide(): void {
        this._document.body.classList.add('treo-splash-screen-hidden');
    }
}
