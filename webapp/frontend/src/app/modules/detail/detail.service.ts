import {Injectable} from '@angular/core';
import { HttpClient } from '@angular/common/http';
import {BehaviorSubject, Observable} from 'rxjs';
import {tap} from 'rxjs/operators';
import {getBasePath} from 'app/app.routing';
import {DeviceDetailsResponseWrapper} from 'app/core/models/device-details-response-wrapper';
import {PerformanceResponseWrapper} from 'app/core/models/measurements/performance-model';
import {ReplacementRiskResponseWrapper} from 'app/core/models/replacement-risk-model';

@Injectable({
    providedIn: 'root'
})
export class DetailService {
    // Observables
    private _data: BehaviorSubject<DeviceDetailsResponseWrapper>;

    /**
     * Constructor
     *
     * @param {HttpClient} _httpClient
     */
    constructor(
        private readonly _httpClient: HttpClient
    ) {
        // Set the private defaults
        this._data = new BehaviorSubject(null);
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Accessors
    // -----------------------------------------------------------------------------------------------------

    /**
     * Getter for data
     */
    get data$(): Observable<DeviceDetailsResponseWrapper> {
        return this._data.asObservable();
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    /**
     * Get data
     */
    getData(deviceId): Observable<DeviceDetailsResponseWrapper> {
        return this._httpClient.get(getBasePath() + `/api/device/${deviceId}/details`).pipe(
            tap((response: DeviceDetailsResponseWrapper) => {
                this._data.next(response);
            })
        );
    }

    /**
     * Reset device failed status to passed
     */
    resetStatus(deviceId: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/device/${deviceId}/reset-status`, {});
    }

    /**
     * Mute / Unmute certain device
     */
    setMuted(deviceId, muted): Observable<any> {
        const action = muted ? 'mute' : 'unmute';
        return this._httpClient.post(getBasePath() + `/api/device/${deviceId}/${action}`, {});
    }

    /**
     * Set device label (custom user-provided name)
     */
    setLabel(deviceId: string, label: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/device/${deviceId}/label`, { label });
    }

    setSmartDisplayMode(deviceId: string, mode: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/device/${deviceId}/smart-display-mode`, { smart_display_mode: mode });
    }

    setMissedPingTimeout(deviceId: string, timeoutMinutes: number): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/device/${deviceId}/missed-ping-timeout`, { missed_ping_timeout_override: timeoutMinutes });
    }

    getPerformanceData(deviceId: string, duration: string = 'week'): Observable<PerformanceResponseWrapper> {
        const params = { duration };
        return this._httpClient.get<PerformanceResponseWrapper>(
            getBasePath() + `/api/device/${deviceId}/performance`,
            { params }
        );
    }

    getReplacementRisk(deviceId: string, trendWindow: string = '30d'): Observable<ReplacementRiskResponseWrapper> {
        const params = { trend_window: trendWindow };
        return this._httpClient.get<ReplacementRiskResponseWrapper>(
            getBasePath() + `/api/device/${deviceId}/replacement-risk`,
            { params }
        );
    }
}
