import { Route } from '@angular/router';
import { BtrfsFilesystemsComponent } from 'app/modules/btrfs-filesystems/btrfs-filesystems.component';
import { BtrfsFilesystemsResolver } from 'app/modules/btrfs-filesystems/btrfs-filesystems.resolvers';

export const btrfsFilesystemsRoutes: Route[] = [
    {
        path: '',
        component: BtrfsFilesystemsComponent,
        resolve: {
            filesystems: BtrfsFilesystemsResolver
        }
    }
];
