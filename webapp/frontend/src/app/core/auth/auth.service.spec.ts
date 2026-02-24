import { HttpClient } from '@angular/common/http';
import { Router } from '@angular/router';
import { of, throwError } from 'rxjs';
import { AuthService } from './auth.service';

describe('AuthService', () => {
    let service: AuthService;
    let httpSpy: jasmine.SpyObj<HttpClient>;
    let routerSpy: jasmine.SpyObj<Router>;

    beforeEach(() => {
        httpSpy = jasmine.createSpyObj('HttpClient', ['get', 'post']);
        routerSpy = jasmine.createSpyObj('Router', ['navigate']);
        service = new AuthService(httpSpy, routerSpy);
        localStorage.clear();
    });

    afterEach(() => {
        localStorage.clear();
    });

    describe('init()', () => {
        it('should set authEnabled=true when server reports auth enabled', async () => {
            httpSpy.get.and.returnValue(of({ success: true, auth_enabled: true, login_methods: ['token', 'password'] }));
            await service.init();
            expect(service.authEnabled).toBeTrue();
        });

        it('should set authEnabled=false when server reports auth disabled', async () => {
            httpSpy.get.and.returnValue(of({ success: true, auth_enabled: false }));
            await service.init();
            expect(service.authEnabled).toBeFalse();
            expect(service.isLoggedIn).toBeTrue();
        });

        it('should default to auth disabled on network error', async () => {
            httpSpy.get.and.returnValue(throwError(() => new Error('Network error')));
            await service.init();
            expect(service.authEnabled).toBeFalse();
            expect(service.isLoggedIn).toBeTrue();
        });
    });

    describe('isTokenExpired()', () => {
        // Access private method via bracket notation for testing
        function makeJwt(payload: object): string {
            const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
            const body = btoa(JSON.stringify(payload));
            return `${header}.${body}.fakesig`;
        }

        it('should return false for a token expiring in the future', () => {
            const futureExp = Math.floor(Date.now() / 1000) + 3600; // +1 hour
            const token = makeJwt({ exp: futureExp });
            localStorage.setItem('scrutiny_jwt_token', token);
            expect(service.getToken()).not.toBeNull();
        });

        it('should return null for an expired token', () => {
            const pastExp = Math.floor(Date.now() / 1000) - 3600; // -1 hour
            const token = makeJwt({ exp: pastExp });
            localStorage.setItem('scrutiny_jwt_token', token);
            expect(service.getToken()).toBeNull();
        });

        it('should return null for a token expiring within 30 seconds (early expiry buffer)', () => {
            const nearExp = Math.floor(Date.now() / 1000) + 10; // +10 seconds
            const token = makeJwt({ exp: nearExp });
            localStorage.setItem('scrutiny_jwt_token', token);
            expect(service.getToken()).toBeNull();
        });

        it('should return null for a malformed token', () => {
            localStorage.setItem('scrutiny_jwt_token', 'not-a-jwt');
            expect(service.getToken()).toBeNull();
        });
    });

    describe('logout()', () => {
        it('should clear token and navigate to /login', () => {
            localStorage.setItem('scrutiny_jwt_token', 'some-token');
            service.logout();
            expect(localStorage.getItem('scrutiny_jwt_token')).toBeNull();
            expect(service.isLoggedIn).toBeFalse();
            expect(routerSpy.navigate).toHaveBeenCalledWith(['/login']);
        });
    });

    describe('handleUnauthorized()', () => {
        it('should clear token and navigate to /login with returnUrl', () => {
            localStorage.setItem('scrutiny_jwt_token', 'some-token');
            // Mock the router.url property
            Object.defineProperty(routerSpy, 'url', { value: '/device/test', writable: true });
            service.handleUnauthorized();
            expect(localStorage.getItem('scrutiny_jwt_token')).toBeNull();
            expect(service.isLoggedIn).toBeFalse();
            expect(routerSpy.navigate).toHaveBeenCalledWith(['/login'], { queryParams: { returnUrl: '/device/test' } });
        });

        it('should not navigate if already on /login', () => {
            Object.defineProperty(routerSpy, 'url', { value: '/login', writable: true });
            service.handleUnauthorized();
            expect(routerSpy.navigate).not.toHaveBeenCalled();
        });
    });
});
