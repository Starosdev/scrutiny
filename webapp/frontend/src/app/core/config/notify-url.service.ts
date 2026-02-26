import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { getBasePath } from '../../app.routing';
import { NotifyUrlEntry } from './app.config';

interface NotifyUrlsResponse {
    success: boolean;
    data: NotifyUrlEntry[];
}

interface NotifyUrlResponse {
    success: boolean;
    data: NotifyUrlEntry;
}

interface SimpleResponse {
    success: boolean;
}

@Injectable({
    providedIn: 'root'
})
export class NotifyUrlService {
    constructor(private http: HttpClient) {}

    getNotifyUrls(): Observable<NotifyUrlEntry[]> {
        return this.http.get<NotifyUrlsResponse>(
            getBasePath() + '/api/settings/notify-urls'
        ).pipe(map(response => response.data || []));
    }

    addNotifyUrl(url: string, label: string): Observable<NotifyUrlEntry> {
        return this.http.post<NotifyUrlResponse>(
            getBasePath() + '/api/settings/notify-urls',
            { url, label }
        ).pipe(map(response => response.data));
    }

    deleteNotifyUrl(id: number): Observable<void> {
        return this.http.delete<SimpleResponse>(
            getBasePath() + '/api/settings/notify-urls/' + id
        ).pipe(map(() => undefined));
    }

    testNotifyUrl(id: number): Observable<void> {
        return this.http.post<SimpleResponse>(
            getBasePath() + '/api/settings/notify-urls/' + id + '/test',
            {}
        ).pipe(map(() => undefined));
    }
}
