import { appRoutes } from './app.routing';

describe('appRoutes', () => {
    it('should expose a dedicated mobile drives route', () => {
        const protectedRoute = appRoutes.find((route) => route.children?.some((child) => child.path === 'dashboard'));

        expect(protectedRoute?.children?.some((child) => child.path === 'mobile-drives')).toBeTrue();
    });
});
