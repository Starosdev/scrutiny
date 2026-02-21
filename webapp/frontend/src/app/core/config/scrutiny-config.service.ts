import {Inject, Injectable} from '@angular/core';
import { HttpClient } from '@angular/common/http';
import {TREO_APP_CONFIG} from '@treo/services/config/config.constants';
import {BehaviorSubject, Observable} from 'rxjs';
import {getBasePath} from '../../app.routing';
import {map, tap} from 'rxjs/operators';
import {AppConfig} from './app.config';
import {merge} from 'lodash';

@Injectable({
    providedIn: 'root'
})
export class ScrutinyConfigService {
    // Private
    private _config: BehaviorSubject<AppConfig>;
    private _defaultConfig: AppConfig;

    constructor(
        private _httpClient: HttpClient,
        @Inject(TREO_APP_CONFIG) defaultConfig: AppConfig
    ) {
        // Set the private defaults
        this._defaultConfig = defaultConfig
        this._config = new BehaviorSubject(null);
    }


    // -----------------------------------------------------------------------------------------------------
    // @ Accessors
    // -----------------------------------------------------------------------------------------------------

    /**
     * Setter & getter for config
     */
    set config(value: AppConfig) {
        // get the current config, merge the new values, and then submit. (setTheme only sets a single key, not the whole obj)
        const mergedSettings = merge({}, this._config.getValue(), value);

        this._httpClient.post(getBasePath() + '/api/settings', mergedSettings).pipe(
            map((response: any) => {
                return response.settings
            }),
            tap((settings: AppConfig) => {
                this._config.next(settings);
                return settings
            })
        ).subscribe()
    }

    get config$(): Observable<AppConfig> {
        if (this._config.getValue()) {
            return this._config.asObservable()
        } else {
            return this._httpClient.get(getBasePath() + '/api/settings').pipe(
                map((response: any) => {
                    const merged = this._mergeWithDefaults(this._defaultConfig, response.settings);
                    return { ...merged, server_version: response.server_version };
                }),
                tap((settings: AppConfig) => {
                    this._config.next(settings);
                    return this._config.asObservable()
                })
            );
        }

    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    /**
     * Resets the config to the default
     */
    reset(): void {
        // Set the config
        this.config = this._defaultConfig
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Private methods
    // -----------------------------------------------------------------------------------------------------

    /**
     * Deep-merges API settings over defaults, treating empty strings, null,
     * and undefined as "missing" (keeps the default value).
     * Cannot use lodash merge because it treats "" as a valid override.
     */
    private _mergeWithDefaults(defaults: any, api: any): any {
        if (!api || typeof api !== 'object') {
            return { ...defaults };
        }
        const result = { ...defaults };

        // For keys in defaults: use API value unless it's empty/null/undefined
        for (const key of Object.keys(result)) {
            if (!(key in api)) {
                continue;
            }
            const apiVal = api[key];
            const defVal = result[key];

            // Recurse into nested objects (but not arrays)
            if (defVal && typeof defVal === 'object' && !Array.isArray(defVal)
                && apiVal && typeof apiVal === 'object' && !Array.isArray(apiVal)) {
                result[key] = this._mergeWithDefaults(defVal, apiVal);
            } else if (apiVal === '' || apiVal === null || apiVal === undefined) {
                // Keep the default — API value is empty/missing
            } else {
                result[key] = apiVal;
            }
        }

        // Carry over API keys not present in defaults (e.g. when defaults is partial)
        for (const key of Object.keys(api)) {
            if (!(key in result)) {
                result[key] = api[key];
            }
        }

        return result;
    }
}
