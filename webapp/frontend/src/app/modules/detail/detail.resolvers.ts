import {Injectable} from '@angular/core';
import { ActivatedRouteSnapshot, Router, RouterStateSnapshot } from '@angular/router';
import {Observable, of} from 'rxjs';
import {catchError} from 'rxjs/operators';
import {DetailService} from 'app/modules/detail/detail.service';
import {DeviceDetailsResponseWrapper} from 'app/core/models/device-details-response-wrapper';

@Injectable({
    providedIn: 'root'
})
export class DetailResolver  {
    constructor(
        private _detailService: DetailService,
        private _router: Router
    )
    {
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<DeviceDetailsResponseWrapper> {
        return this._detailService.getData(route.params.wwn).pipe(
            catchError((error) => {
                console.error('Failed to load device details:', error);
                this._router.navigate(['/']);
                return of(null);
            })
        );
    }
}
