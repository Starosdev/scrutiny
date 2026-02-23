import { Injectable } from '@angular/core';
import { CanActivate, ActivatedRouteSnapshot, RouterStateSnapshot, Router, UrlTree } from '@angular/router';
import { AuthService } from './auth.service';

@Injectable({ providedIn: 'root' })
export class AuthGuard implements CanActivate {

    constructor(
        private _authService: AuthService,
        private _router: Router
    ) {}

    canActivate(_route: ActivatedRouteSnapshot, state: RouterStateSnapshot): boolean | UrlTree {
        // If auth is not enabled, always allow access
        if (!this._authService.authEnabled) {
            return true;
        }

        // If user is authenticated, allow access
        if (this._authService.isLoggedIn) {
            return true;
        }

        // Redirect to login with returnUrl so user lands back here after login
        return this._router.createUrlTree(['/login'], {
            queryParams: { returnUrl: state.url }
        });
    }
}
