import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Router } from '@angular/router';
import { BehaviorSubject, Observable, of, firstValueFrom } from 'rxjs';
import { tap, catchError } from 'rxjs/operators';
import { getBasePath } from 'app/app.routing';

const TOKEN_KEY = 'scrutiny_jwt_token';

interface AuthStatusResponse {
    success: boolean;
    auth_enabled: boolean;
    login_methods?: string[];
}

interface LoginResponse {
    success: boolean;
    token?: string;
    expires_at?: string;
    token_type?: string;
    auth_enabled?: boolean;
    error?: string;
}

@Injectable({ providedIn: 'root' })
export class AuthService {
    private _authEnabled = new BehaviorSubject<boolean>(false);
    private _isLoggedIn = new BehaviorSubject<boolean>(false);
    private _loginMethods = new BehaviorSubject<string[]>([]);
    private _initialized = false;

    readonly authEnabled$ = this._authEnabled.asObservable();
    readonly isLoggedIn$ = this._isLoggedIn.asObservable();
    readonly loginMethods$ = this._loginMethods.asObservable();

    constructor(
        private _http: HttpClient,
        private _router: Router
    ) {}

    get authEnabled(): boolean {
        return this._authEnabled.value;
    }

    get isLoggedIn(): boolean {
        return this._isLoggedIn.value;
    }

    // Called by provideAppInitializer before routing starts
    init(): Promise<void> {
        const source$ = this._http.get<AuthStatusResponse>(getBasePath() + '/api/auth/status')
            .pipe(
                tap((res) => {
                    this._authEnabled.next(res.auth_enabled);
                    this._loginMethods.next(res.login_methods || []);
                    this._initialized = true;

                    if (res.auth_enabled) {
                        const token = this.getToken();
                        this._isLoggedIn.next(token !== null && !this.isTokenExpired(token));
                    } else {
                        this._isLoggedIn.next(true);
                    }
                }),
                catchError(() => {
                    // If auth status check fails, assume auth is disabled for backward compat
                    this._authEnabled.next(false);
                    this._isLoggedIn.next(true);
                    this._initialized = true;
                    return of(undefined);
                })
            );

        return firstValueFrom(source$).then(() => {});
    }

    loginWithToken(token: string): Observable<LoginResponse> {
        return this._http.post<LoginResponse>(getBasePath() + '/api/auth/login', { token })
            .pipe(tap((res) => this.handleLoginResponse(res)));
    }

    loginWithPassword(username: string, password: string): Observable<LoginResponse> {
        return this._http.post<LoginResponse>(getBasePath() + '/api/auth/login', { username, password })
            .pipe(tap((res) => this.handleLoginResponse(res)));
    }

    logout(): void {
        localStorage.removeItem(TOKEN_KEY);
        this._isLoggedIn.next(false);
        this._router.navigate(['/login']);
    }

    getToken(): string | null {
        const token = localStorage.getItem(TOKEN_KEY);
        if (token && this.isTokenExpired(token)) {
            localStorage.removeItem(TOKEN_KEY);
            return null;
        }
        return token;
    }

    // Called by the interceptor when a 401 is received
    handleUnauthorized(): void {
        localStorage.removeItem(TOKEN_KEY);
        this._isLoggedIn.next(false);
        if (this._router.url !== '/login') {
            this._router.navigate(['/login'], { queryParams: { returnUrl: this._router.url } });
        }
    }

    private handleLoginResponse(res: LoginResponse): void {
        if (res.success && res.token) {
            localStorage.setItem(TOKEN_KEY, res.token);
            this._isLoggedIn.next(true);
        }
    }

    private isTokenExpired(token: string): boolean {
        try {
            const payload = JSON.parse(atob(token.split('.')[1]));
            if (!payload.exp) {
                return false;
            }
            // Expire 30s early to avoid edge-case expiry during request
            return (payload.exp * 1000) < (Date.now() + 30000);
        } catch {
            return true;
        }
    }
}
