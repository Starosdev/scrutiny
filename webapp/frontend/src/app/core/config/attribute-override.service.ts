import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { getBasePath } from '../../app.routing';
import { AttributeOverride } from './app.config';

interface OverridesResponse {
    success: boolean;
    data: AttributeOverride[];
}

interface OverrideResponse {
    success: boolean;
    data: AttributeOverride;
}

interface DeleteResponse {
    success: boolean;
}

@Injectable({
    providedIn: 'root'
})
export class AttributeOverrideService {
    constructor(private http: HttpClient) {}

    /**
     * Get all attribute overrides from the database
     */
    getOverrides(): Observable<AttributeOverride[]> {
        return this.http.get<OverridesResponse>(
            getBasePath() + '/api/settings/overrides'
        ).pipe(map(response => response.data || []));
    }

    /**
     * Save (create or update) an attribute override
     */
    saveOverride(override: AttributeOverride): Observable<AttributeOverride> {
        return this.http.post<OverrideResponse>(
            getBasePath() + '/api/settings/overrides',
            override
        ).pipe(map(response => response.data));
    }

    /**
     * Delete an attribute override by ID
     */
    deleteOverride(id: number): Observable<void> {
        return this.http.delete<DeleteResponse>(
            getBasePath() + '/api/settings/overrides/' + id
        ).pipe(map(() => undefined));
    }
}
