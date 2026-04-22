import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, Observable } from 'rxjs';
import { map, tap } from 'rxjs/operators';
import { getBasePath } from 'app/app.routing';
import { MDADMArrayModel, MDADMArrayResponseWrapper, MDADMArrayDetailResponseWrapper } from 'app/core/models/mdadm-array-model';

@Injectable({
    providedIn: 'root',
})
export class MDADMService {
    // Observables
    private _data: BehaviorSubject<MDADMArrayModel[]>;

    constructor(private readonly _httpClient: HttpClient) {
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
                return response.data;
            }),
            tap((response: MDADMArrayModel[]) => {
                this._data.next(response);
            })
        );
    }

    getDetails(uuid: string, duration: string = 'week'): Observable<MDADMArrayDetailResponseWrapper> {
        return this._httpClient.get<MDADMArrayDetailResponseWrapper>(getBasePath() + `/api/mdadm/array/${uuid}/details?duration=${duration}`);
    }

    // Reuse ZFS-like actions if implemented in backend (placeholder for future expansion)
    // archiveArray, muteArray, setLabel, deleteArray
}
