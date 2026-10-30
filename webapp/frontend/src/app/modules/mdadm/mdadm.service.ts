import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, Observable } from 'rxjs';
import { map, tap } from 'rxjs/operators';
import { getBasePath } from 'app/app.routing';
import { MDADMArrayModel, MDADMArrayResponseWrapper, MDADMArrayDetailResponseWrapper } from 'app/core/models/mdadm-array-model';

@Injectable({
    providedIn: 'root',
})
export class MDADMService {
    private readonly _httpClient = inject(HttpClient);

    // Observables
    private readonly _data: BehaviorSubject<MDADMArrayModel[]>;

    constructor() {
        this._data = new BehaviorSubject(null);
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Accessors
    // -----------------------------------------------------------------------------------------------------

    get data$(): Observable<MDADMArrayModel[]> {
        return this._data.asObservable();
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    getSummaryData(): Observable<MDADMArrayModel[]> {
        return this._httpClient.get(getBasePath() + '/api/mdadm/summary').pipe(
            map((response: MDADMArrayResponseWrapper) => {
                return response.data.map((array) => ({
                    ...array,
                    devices: array.devices ?? [],
                }));
            }),
            tap((response: MDADMArrayModel[]) => {
                this._data.next(response);
            })
        );
    }

    getDetails(uuid: string, duration: string = 'week'): Observable<MDADMArrayDetailResponseWrapper> {
        return this._httpClient.get<MDADMArrayDetailResponseWrapper>(getBasePath() + `/api/mdadm/array/${encodeURIComponent(uuid)}/details?duration=${duration}`);
    }

    archiveArray(uuid: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/mdadm/array/${encodeURIComponent(uuid)}/archive`, {});
    }

    unarchiveArray(uuid: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/mdadm/array/${encodeURIComponent(uuid)}/unarchive`, {});
    }

    muteArray(uuid: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/mdadm/array/${encodeURIComponent(uuid)}/mute`, {});
    }

    unmuteArray(uuid: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/mdadm/array/${encodeURIComponent(uuid)}/unmute`, {});
    }

    setLabel(uuid: string, label: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/mdadm/array/${encodeURIComponent(uuid)}/label`, { label });
    }

    deleteArray(uuid: string): Observable<any> {
        return this._httpClient.delete(getBasePath() + `/api/mdadm/array/${encodeURIComponent(uuid)}`);
    }
}
