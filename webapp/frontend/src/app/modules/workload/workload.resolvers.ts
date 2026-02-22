import { Injectable } from '@angular/core';
import { ActivatedRouteSnapshot, RouterStateSnapshot } from '@angular/router';
import { Observable } from 'rxjs';
import { WorkloadService } from 'app/modules/workload/workload.service';
import { WorkloadInsightModel } from 'app/core/models/workload-insight-model';

@Injectable({
    providedIn: 'root',
})
export class WorkloadResolver {
    constructor(private _workloadService: WorkloadService) {}

    resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<Record<string, WorkloadInsightModel>> {
        return this._workloadService.getWorkloadData();
    }
}
