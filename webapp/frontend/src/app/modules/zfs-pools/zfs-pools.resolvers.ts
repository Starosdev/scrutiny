import { Injectable, inject } from '@angular/core';
import { ActivatedRouteSnapshot, RouterStateSnapshot } from '@angular/router';
import { Observable } from 'rxjs';
import { ZFSPoolsService } from 'app/modules/zfs-pools/zfs-pools.service';
import { ZFSPoolModel } from 'app/core/models/zfs-pool-model';

@Injectable({
    providedIn: 'root',
})
export class ZFSPoolsResolver {
    private readonly _zfsPoolsService = inject(ZFSPoolsService);

    resolve(_route: ActivatedRouteSnapshot, _state: RouterStateSnapshot): Observable<Record<string, ZFSPoolModel>> {
        return this._zfsPoolsService.getSummaryData();
    }
}
