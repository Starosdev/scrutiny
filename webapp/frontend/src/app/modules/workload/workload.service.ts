import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, Observable } from 'rxjs';
import { map, tap } from 'rxjs/operators';
import { getBasePath } from 'app/app.routing';
import { WorkloadInsightModel, WorkloadResponseWrapper } from 'app/core/models/workload-insight-model';

@Injectable({
    providedIn: 'root',
})
export class WorkloadService {
    private _data: BehaviorSubject<Record<string, WorkloadInsightModel>>;

    constructor(private _httpClient: HttpClient) {
        this._data = new BehaviorSubject(null);
    }

    get data$(): Observable<Record<string, WorkloadInsightModel>> {
        return this._data.asObservable();
    }

    getWorkloadData(durationKey: string = 'week'): Observable<Record<string, WorkloadInsightModel>> {
        const params: any = {};
        if (durationKey) {
            params.duration_key = durationKey;
        }

        return this._httpClient.get(getBasePath() + '/api/summary/workload', { params }).pipe(
            map((response: WorkloadResponseWrapper) => {
                return response.data.workload;
            }),
            tap((response: Record<string, WorkloadInsightModel>) => {
                this._data.next(response);
            })
        );
    }
}
