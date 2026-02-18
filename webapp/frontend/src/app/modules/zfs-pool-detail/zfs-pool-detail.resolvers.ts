import { Injectable } from '@angular/core';
import { ActivatedRouteSnapshot, Router, RouterStateSnapshot } from '@angular/router';
import { Observable, of } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { ZFSPoolDetailService } from 'app/modules/zfs-pool-detail/zfs-pool-detail.service';
import { ZFSPoolDetailsResponseWrapper } from 'app/core/models/zfs-pool-summary-model';

@Injectable({
    providedIn: 'root'
})
export class ZFSPoolDetailResolver {
    constructor(
        private _zfsPoolDetailService: ZFSPoolDetailService,
        private _router: Router
    ) {
    }

    resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<ZFSPoolDetailsResponseWrapper> {
        return this._zfsPoolDetailService.getData(route.params.guid).pipe(
            catchError((error) => {
                console.error('Failed to load ZFS pool details:', error);
                this._router.navigate(['/zfs-pools']);
                return of(null);
            })
        );
    }
}
