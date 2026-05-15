import { Route } from '@angular/router';
import { BtrfsFilesystemDetailComponent } from 'app/modules/btrfs-filesystem-detail/btrfs-filesystem-detail.component';
import { BtrfsFilesystemDetailResolver } from './btrfs-filesystem-detail.resolvers';

export const btrfsFilesystemDetailRoutes: Route[] = [
    {
        path: '',
        component: BtrfsFilesystemDetailComponent,
        resolve: {
            filesystem: BtrfsFilesystemDetailResolver
        }
    }
];
