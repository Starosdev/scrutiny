import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { BehaviorSubject, Observable } from 'rxjs';
import { map, tap } from 'rxjs/operators';
import { getBasePath } from 'app/app.routing';
import { DeviceSummaryPage, DeviceSummaryResponseWrapper } from 'app/core/models/device-summary-response-wrapper';
import { DeviceSummaryModel } from 'app/core/models/device-summary-model';
import { SmartTemperatureModel } from 'app/core/models/measurements/smart-temperature-model';
import { DeviceSummaryTempResponseWrapper, TemperatureDeviceOption, TemperatureDeviceOptionsResponseWrapper } from 'app/core/models/device-summary-temp-response-wrapper';
import { FilesystemSummaryResponseWrapper, FilesystemCapacityModel, FilesystemHostStatusModel } from 'app/core/models/filesystem-summary-model';
import { DashboardDisplay, DashboardHostPageSize, DashboardPageSize, DashboardSort } from 'app/core/config/app.config';
import { TemperatureSelection } from 'app/modules/dashboard/temperature-selection';

export interface SummaryPageRequest {
    page: number;
    pageSize?: DashboardPageSize | DashboardHostPageSize;
    groupBy?: 'host';
    archived?: boolean;
    sort?: DashboardSort;
    display?: DashboardDisplay;
    hostSearch?: string;
}

@Injectable({
    providedIn: 'root',
})
export class DashboardService {
    private readonly _httpClient = inject(HttpClient);

    // Observables
    private readonly _data: BehaviorSubject<{ [p: string]: DeviceSummaryModel }>;
    private readonly _pageData: BehaviorSubject<DeviceSummaryPage | null>;
    readonly temperatureSelection = new TemperatureSelection();

    /**
     * Constructor
     *
     * @param {HttpClient} _httpClient
     */
    constructor() {
        // Set the private defaults
        this._data = new BehaviorSubject(null);
        this._pageData = new BehaviorSubject(null);
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Accessors
    // -----------------------------------------------------------------------------------------------------

    /**
     * Getter for data
     */
    get data$(): Observable<{ [p: string]: DeviceSummaryModel }> {
        return this._data.asObservable();
    }

    get pageData$(): Observable<DeviceSummaryPage | null> {
        return this._pageData.asObservable();
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    /**
     * Get data
     */
    getSummaryData(): Observable<{ [key: string]: DeviceSummaryModel }> {
        return this._httpClient.get(getBasePath() + '/api/summary').pipe(
            map((response: DeviceSummaryResponseWrapper) => {
                return response.data.summary;
            }),
            tap((response: { [key: string]: DeviceSummaryModel }) => {
                this._data.next(response);
            })
        );
    }

    getSummaryPage(request: SummaryPageRequest): Observable<DeviceSummaryPage> {
        const params: Record<string, string> = {
            page: request.page.toString(),
            archived: String(request.archived ?? false),
        };
        if (request.pageSize) {
            params['page_size'] = request.pageSize.toString();
        }
        if (request.groupBy) {
            params['group_by'] = request.groupBy;
        }
        if (request.sort) {
            params['sort'] = request.sort;
        }
        if (request.display) {
            params['display'] = request.display;
        }
        if (request.hostSearch) {
            params['host'] = request.hostSearch;
        }

        return this._httpClient.get<DeviceSummaryResponseWrapper>(getBasePath() + '/api/summary', { params }).pipe(
            map((response) => {
                return {
                    summary: response.data.summary,
                    pagination: response.data.pagination,
                } as DeviceSummaryPage;
            }),
            tap((response) => {
                this._pageData.next(response);
            })
        );
    }

    getSummaryTempData(durationKey: string, deviceIDs?: string[]): Observable<{ [key: string]: SmartTemperatureModel[] }> {
        let params = new HttpParams();
        if (durationKey) {
            params = params.set('duration_key', durationKey);
        }
        for (const deviceID of deviceIDs ?? []) {
            params = params.append('device_id', deviceID);
        }

        return this._httpClient.get(getBasePath() + '/api/summary/temp', { params }).pipe(
            map((response: DeviceSummaryTempResponseWrapper) => {
                return response.data.temp_history;
            })
        );
    }

    getTemperatureDeviceOptions(): Observable<TemperatureDeviceOption[]> {
        return this._httpClient.get<TemperatureDeviceOptionsResponseWrapper>(getBasePath() + '/api/summary/temp/devices').pipe(map((response) => response.data.devices));
    }

    runCollectors(): Observable<any> {
        return this._httpClient.post(getBasePath() + '/api/collectors/run', {});
    }

    getCollectorRunStatus(): Observable<boolean> {
        return this._httpClient.get<{ running: boolean }>(getBasePath() + '/api/collectors/run').pipe(map((response) => response.running));
    }
    getFilesystemSummaryData(): Observable<{ filesystems: Record<string, FilesystemCapacityModel[]>; hosts: Record<string, FilesystemHostStatusModel> }> {
        return this._httpClient.get(getBasePath() + '/api/filesystems/summary').pipe(
            map((response: FilesystemSummaryResponseWrapper) => {
                return response.data;
            })
        );
    }
}
