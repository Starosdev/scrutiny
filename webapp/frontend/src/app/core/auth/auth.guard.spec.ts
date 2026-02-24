import { Router } from '@angular/router';
import { AuthGuard } from './auth.guard';
import { AuthService } from './auth.service';

describe('AuthGuard', () => {
    let guard: AuthGuard;
    let authServiceSpy: jasmine.SpyObj<AuthService>;
    let routerSpy: jasmine.SpyObj<Router>;

    beforeEach(() => {
        authServiceSpy = jasmine.createSpyObj('AuthService', [], {
            authEnabled: false,
            isLoggedIn: false
        });
        routerSpy = jasmine.createSpyObj('Router', ['createUrlTree']);
        routerSpy.createUrlTree.and.returnValue({} as any);
        guard = new AuthGuard(authServiceSpy, routerSpy);
    });

    it('should allow access when auth is disabled', () => {
        (Object.getOwnPropertyDescriptor(authServiceSpy, 'authEnabled')!.get as jasmine.Spy).and.returnValue(false);
        const mockState = { url: '/dashboard' } as any;
        expect(guard.canActivate({} as any, mockState)).toBeTrue();
    });

    it('should allow access when user is logged in', () => {
        (Object.getOwnPropertyDescriptor(authServiceSpy, 'authEnabled')!.get as jasmine.Spy).and.returnValue(true);
        (Object.getOwnPropertyDescriptor(authServiceSpy, 'isLoggedIn')!.get as jasmine.Spy).and.returnValue(true);
        const mockState = { url: '/dashboard' } as any;
        expect(guard.canActivate({} as any, mockState)).toBeTrue();
    });

    it('should redirect to /login with returnUrl when not authenticated', () => {
        (Object.getOwnPropertyDescriptor(authServiceSpy, 'authEnabled')!.get as jasmine.Spy).and.returnValue(true);
        (Object.getOwnPropertyDescriptor(authServiceSpy, 'isLoggedIn')!.get as jasmine.Spy).and.returnValue(false);
        const mockState = { url: '/device/abc' } as any;
        guard.canActivate({} as any, mockState);
        expect(routerSpy.createUrlTree).toHaveBeenCalledWith(['/login'], {
            queryParams: { returnUrl: '/device/abc' }
        });
    });
});
