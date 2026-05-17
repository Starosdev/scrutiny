import { Injectable, inject } from '@angular/core';
import { ActivatedRouteSnapshot, RouterStateSnapshot } from '@angular/router';
import { Observable } from 'rxjs';
import { WorkloadService } from 'app/modules/workload/workload.service';
import { WorkloadInsightModel } from 'app/core/models/workload-insight-model';

@Injectable({
    providedIn: 'root',
})
export class WorkloadResolver {
    private readonly _workloadService = inject(WorkloadService);

    resolve(_route: ActivatedRouteSnapshot, _state: RouterStateSnapshot): Observable<Record<string, WorkloadInsightModel>> {
        return this._workloadService.getWorkloadData();
    }
}
