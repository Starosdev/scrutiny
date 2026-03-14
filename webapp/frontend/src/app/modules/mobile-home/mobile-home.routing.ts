import { Route } from '@angular/router';
import { MobileHomeComponent } from './mobile-home.component';
import { MobileHomeResolver } from './mobile-home.resolver';

export const mobileHomeRoutes: Route[] = [
    {
        path: '',
        component: MobileHomeComponent,
        resolve: {
            data: MobileHomeResolver
        }
    }
];
