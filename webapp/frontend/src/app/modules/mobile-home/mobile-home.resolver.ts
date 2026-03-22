import { Injectable } from '@angular/core';
import { Resolve } from '@angular/router';
import { forkJoin, Observable, of } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { DashboardService } from 'app/modules/dashboard/dashboard.service';
import { ZFSPoolsService } from 'app/modules/zfs-pools/zfs-pools.service';

@Injectable({
    providedIn: 'root'
})
export class MobileHomeResolver implements Resolve<any> {
    constructor(
        private readonly _dashboardService: DashboardService,
        private readonly _zfsPoolsService: ZFSPoolsService
    ) {}

    resolve(): Observable<any> {
        return forkJoin({
            smart: this._dashboardService.getSummaryData(),
            zfs: this._zfsPoolsService.getSummaryData().pipe(
                catchError(() => of({}))
            )
        });
    }
}
