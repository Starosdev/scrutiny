import { Injectable, inject } from '@angular/core';
import { ActivatedRouteSnapshot, Router, RouterStateSnapshot } from '@angular/router';
import { Observable, of } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { DetailService } from 'app/modules/detail/detail.service';
import { DeviceDetailsResponseWrapper } from 'app/core/models/device-details-response-wrapper';

@Injectable({
    providedIn: 'root',
})
export class DetailResolver {
    private readonly _detailService = inject(DetailService);
    private readonly _router = inject(Router);

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<DeviceDetailsResponseWrapper> {
        return this._detailService.getData(route.params.device_id).pipe(
            catchError((error) => {
                console.error('Failed to load device details:', error);
                this._router.navigate(['/']);
                return of(null);
            })
        );
    }
}
