import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, Observable } from 'rxjs';
import { map, tap } from 'rxjs/operators';
import { getBasePath } from 'app/app.routing';
import { BtrfsFilesystemSummaryResponseWrapper } from 'app/core/models/btrfs-filesystem-summary-model';
import { BtrfsFilesystemModel } from 'app/core/models/btrfs-filesystem-model';

@Injectable({
    providedIn: 'root',
})
export class BtrfsFilesystemsService {
    private _data: BehaviorSubject<Record<string, BtrfsFilesystemModel>>;

    constructor(private readonly _httpClient: HttpClient) {
        this._data = new BehaviorSubject(null);
    }

    get data$(): Observable<Record<string, BtrfsFilesystemModel>> {
        return this._data.asObservable();
    }

    getSummaryData(): Observable<Record<string, BtrfsFilesystemModel>> {
        return this._httpClient.get(getBasePath() + '/api/btrfs/summary').pipe(
            map((response: BtrfsFilesystemSummaryResponseWrapper) => response.data.filesystems),
            tap((response: Record<string, BtrfsFilesystemModel>) => this._data.next(response))
        );
    }

    archiveFilesystem(uuid: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/btrfs/filesystem/${uuid}/archive`, {});
    }

    unarchiveFilesystem(uuid: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/btrfs/filesystem/${uuid}/unarchive`, {});
    }

    muteFilesystem(uuid: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/btrfs/filesystem/${uuid}/mute`, {});
    }

    unmuteFilesystem(uuid: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/btrfs/filesystem/${uuid}/unmute`, {});
    }

    deleteFilesystem(uuid: string): Observable<any> {
        return this._httpClient.delete(getBasePath() + `/api/btrfs/filesystem/${uuid}`);
    }

    setLabel(uuid: string, label: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/btrfs/filesystem/${uuid}/label`, { label });
    }
}
