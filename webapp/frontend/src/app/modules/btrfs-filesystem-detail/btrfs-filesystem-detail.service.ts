import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, Observable } from 'rxjs';
import { tap } from 'rxjs/operators';
import { getBasePath } from 'app/app.routing';
import { BtrfsFilesystemDetailsResponseWrapper } from 'app/core/models/btrfs-filesystem-summary-model';

@Injectable({
    providedIn: 'root',
})
export class BtrfsFilesystemDetailService {
    private _data: BehaviorSubject<BtrfsFilesystemDetailsResponseWrapper>;

    constructor(private readonly _httpClient: HttpClient) {
        this._data = new BehaviorSubject(null);
    }

    get data$(): Observable<BtrfsFilesystemDetailsResponseWrapper> {
        return this._data.asObservable();
    }

    getData(uuid: string): Observable<BtrfsFilesystemDetailsResponseWrapper> {
        return this._httpClient
            .get(getBasePath() + `/api/btrfs/filesystem/${uuid}/details`)
            .pipe(tap((response: BtrfsFilesystemDetailsResponseWrapper) => this._data.next(response)));
    }

    setMuted(uuid: string, muted: boolean): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/btrfs/filesystem/${uuid}/${muted ? 'mute' : 'unmute'}`, {});
    }

    setLabel(uuid: string, label: string): Observable<any> {
        return this._httpClient.post(getBasePath() + `/api/btrfs/filesystem/${uuid}/label`, { label });
    }
}
