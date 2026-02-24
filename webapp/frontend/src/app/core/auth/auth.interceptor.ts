import { Injectable } from '@angular/core';
import { HttpInterceptor, HttpRequest, HttpHandler, HttpEvent, HttpErrorResponse } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { AuthService } from './auth.service';

// Paths that should never have auth tokens injected
const AUTH_SKIP_PATHS = ['/api/auth/status', '/api/auth/login'];

@Injectable()
export class AuthInterceptor implements HttpInterceptor {

    constructor(private _authService: AuthService) {}

    intercept(req: HttpRequest<any>, next: HttpHandler): Observable<HttpEvent<any>> {
        let authReq = req;

        // Skip token injection for auth endpoints
        const shouldSkip = AUTH_SKIP_PATHS.some(path => req.url.includes(path));
        if (!shouldSkip) {
            const token = this._authService.getToken();
            if (token) {
                authReq = req.clone({
                    setHeaders: { Authorization: `Bearer ${token}` }
                });
            }
        }

        return next.handle(authReq).pipe(
            catchError((error: HttpErrorResponse) => {
                if (error.status === 401 && !shouldSkip && this._authService.authEnabled) {
                    this._authService.handleUnauthorized();
                }
                return throwError(() => error);
            })
        );
    }
}
