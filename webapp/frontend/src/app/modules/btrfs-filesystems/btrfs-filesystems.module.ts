import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { SharedModule } from 'app/shared/shared.module';
import { BtrfsFilesystemsComponent } from 'app/modules/btrfs-filesystems/btrfs-filesystems.component';
import { btrfsFilesystemsRoutes } from 'app/modules/btrfs-filesystems/btrfs-filesystems.routing';
import { MatButtonModule } from '@angular/material/button';
import { MatDividerModule } from '@angular/material/divider';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSortModule } from '@angular/material/sort';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { BtrfsFilesystemCardModule } from 'app/layout/common/btrfs-filesystem-card/btrfs-filesystem-card.module';

@NgModule({
    declarations: [BtrfsFilesystemsComponent],
    imports: [
        RouterModule.forChild(btrfsFilesystemsRoutes),
        MatButtonModule,
        MatDividerModule,
        MatTooltipModule,
        MatIconModule,
        MatMenuModule,
        MatProgressBarModule,
        MatSortModule,
        MatTableModule,
        SharedModule,
        BtrfsFilesystemCardModule,
    ],
})
export class BtrfsFilesystemsModule {}
