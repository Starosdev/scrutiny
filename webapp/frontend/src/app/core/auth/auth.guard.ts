import { Injectable } from '@angular/core';
import { CanActivate, Router, UrlTree } from '@angular/router';
import { AuthService } from './auth.service';

@Injectable({ providedIn: 'root' })
export class AuthGuard implements CanActivate {

    constructor(
        private _authService: AuthService,
        private _router: Router
    ) {}

    canActivate(): boolean | UrlTree {
        // If auth is not enabled, always allow access
        if (!this._authService.authEnabled) {
            return true;
        }

        // If user is authenticated, allow access
        if (this._authService.isLoggedIn) {
            return true;
        }

        // Redirect to login
        return this._router.createUrlTree(['/login']);
    }
}
