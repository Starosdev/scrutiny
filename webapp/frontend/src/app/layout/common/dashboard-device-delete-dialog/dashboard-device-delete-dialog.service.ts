import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { getBasePath } from 'app/app.routing';

@Injectable({
    providedIn: 'root',
})
export class DashboardDeviceDeleteDialogService {
    private readonly _httpClient = inject(HttpClient);

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    deleteDevice(deviceId: string): Observable<any> {
        return this._httpClient.delete(`${getBasePath()}/api/device/${deviceId}`, {});
    }
}
