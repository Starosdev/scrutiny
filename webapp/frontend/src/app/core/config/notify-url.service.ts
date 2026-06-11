import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { getBasePath } from '../../app.routing';
import { NotifyUrlEntry } from './app.config';

interface NotifyUrlsResponse {
    success: boolean;
    data: any[];
}

interface NotifyUrlResponse {
    success: boolean;
    data: any;
}

interface SimpleResponse {
    success: boolean;
}

@Injectable({
    providedIn: 'root',
})
export class NotifyUrlService {
    private readonly http = inject(HttpClient);

    getNotifyUrls(): Observable<NotifyUrlEntry[]> {
        return this.http.get<NotifyUrlsResponse>(getBasePath() + '/api/settings/notify-urls').pipe(
            map((response) =>
                (response.data || []).map(
                    (e: any) =>
                        ({
                            id: e.id,
                            url: e.url,
                            label: e.label,
                            source: e.source,
                            heartbeatEnabled: e.heartbeat_enabled,
                        } as NotifyUrlEntry)
                )
            )
        );
    }

    addNotifyUrl(url: string, label: string, heartbeatEnabled: boolean = false): Observable<NotifyUrlEntry> {
        return this.http
            .post<NotifyUrlResponse>(getBasePath() + '/api/settings/notify-urls', {
                url,
                label,
                heartbeat_enabled: heartbeatEnabled,
            })
            .pipe(
                map(
                    (response) =>
                        ({
                            id: response.data.id,
                            url: response.data.url,
                            label: response.data.label,
                            source: response.data.source,
                            heartbeatEnabled: response.data.heartbeat_enabled,
                        } as NotifyUrlEntry)
                )
            );
    }

    deleteNotifyUrl(id: number): Observable<void> {
        return this.http.delete<SimpleResponse>(getBasePath() + '/api/settings/notify-urls/' + id).pipe(map(() => undefined));
    }

    testNotifyUrl(id: number): Observable<void> {
        return this.http.post<SimpleResponse>(getBasePath() + '/api/settings/notify-urls/' + id + '/test', {}).pipe(map(() => undefined));
    }

    setHeartbeatEnabled(id: number, enabled: boolean): Observable<void> {
        return this.http.patch<SimpleResponse>(getBasePath() + '/api/settings/notify-urls/' + id, { enabled }).pipe(map(() => void 0));
    }
}
