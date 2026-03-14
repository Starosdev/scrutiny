import { Route } from '@angular/router';
import { LayoutComponent } from 'app/layout/layout.component';
import { EmptyLayoutComponent } from 'app/layout/layouts/empty/empty.component';
import { AuthGuard } from 'app/core/auth/auth.guard';

// @formatter:off
export function getAppBaseHref(): string {
    return getBasePath() + '/web';
}

// @formatter:off
// tslint:disable:max-line-length
export function getBasePath(): string {
    return window.location.pathname.split('/web').slice(0, 1)[0];
}

// @formatter:off
// tslint:disable:max-line-length
export const appRoutes: Route[] = [

    // Redirect empty path to '/example'
    {path: '', pathMatch : 'full', redirectTo: 'dashboard'},


    // Auth & landing routes (no guard, empty layout)
    {
        path: '',
        component: EmptyLayoutComponent,
        children   : [
            {path: 'login', loadChildren: () => import('app/modules/auth/auth.module').then(m => m.AuthModule)},
            {path: 'home', loadChildren: () => import('app/modules/landing/home/home.module').then(m => m.LandingHomeModule)},
        ]
    },

    // Protected routes (guarded when auth is enabled)
    {
        path       : '',
        component  : LayoutComponent,
        canActivate: [AuthGuard],
        children   : [

            // Example
            {path: 'dashboard', loadChildren: () => import('app/modules/dashboard/dashboard.module').then(m => m.DashboardModule)},
            {path: 'device/:device_id', loadChildren: () => import('app/modules/detail/detail.module').then(m => m.DetailModule)},

            // ZFS Pools
            {path: 'zfs-pools', loadChildren: () => import('app/modules/zfs-pools/zfs-pools.module').then(m => m.ZFSPoolsModule)},
            {path: 'zfs-pool/:guid', loadChildren: () => import('app/modules/zfs-pool-detail/zfs-pool-detail.module').then(m => m.ZFSPoolDetailModule)},

            // Workload Insights
            {path: 'workload', loadChildren: () => import('app/modules/workload/workload.module').then(m => m.WorkloadModule)},

            // Mobile-only routes
            {path: 'mobile-home', loadChildren: () => import('app/modules/mobile-home/mobile-home.module').then(m => m.MobileHomeModule)},
            {path: 'mobile-settings', loadChildren: () => import('app/modules/mobile-settings/mobile-settings.module').then(m => m.MobileSettingsModule)}

            // 404 & Catch all
            // {path: '404-not-found', pathMatch: 'full', loadChildren: () => import('app/modules/admin/pages/errors/error-404/error-404.module').then(m => m.Error404Module)},
            // {path: '**', redirectTo: '404-not-found'}
        ]
    }
];
