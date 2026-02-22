import { Route } from '@angular/router';
import { WorkloadComponent } from 'app/modules/workload/workload.component';
import { WorkloadResolver } from 'app/modules/workload/workload.resolvers';

export const workloadRoutes: Route[] = [
    {
        path: '',
        component: WorkloadComponent,
        resolve: {
            workload: WorkloadResolver
        }
    }
];
