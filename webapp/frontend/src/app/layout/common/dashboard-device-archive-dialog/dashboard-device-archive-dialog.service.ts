import {Injectable} from '@angular/core';
import { HttpClient } from '@angular/common/http';
import {Observable} from 'rxjs';
import {getBasePath} from 'app/app.routing';

@Injectable({
    providedIn: 'root'
})
export class DashboardDeviceArchiveDialogService
{


    /**
     * Constructor
     *
     * @param {HttpClient} _httpClient
     */
    constructor(
        private readonly _httpClient: HttpClient
    )
    {
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------


    archiveDevice(deviceId: string): Observable<any>
    {
        return this._httpClient.post( `${getBasePath()}/api/device/${deviceId}/archive`, {});
    }

    unarchiveDevice(deviceId: string): Observable<any>
    {
        return this._httpClient.post( `${getBasePath()}/api/device/${deviceId}/unarchive`, {});
    }
}
