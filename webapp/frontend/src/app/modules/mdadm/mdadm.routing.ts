import { Route } from '@angular/router';
import { MDADMComponent } from 'app/modules/mdadm/mdadm.component';
import { MDADMDetailComponent } from 'app/modules/mdadm/details/mdadm-detail.component';

export const mdadmRoutes: Route[] = [
    {
        path: '',
        component: MDADMComponent
    },
    {
        path: ':uuid',
        component: MDADMDetailComponent
    }
];
