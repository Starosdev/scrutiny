import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { SharedModule } from 'app/shared/shared.module';
import { BtrfsFilesystemDetailComponent } from 'app/modules/btrfs-filesystem-detail/btrfs-filesystem-detail.component';
import { btrfsFilesystemDetailRoutes } from 'app/modules/btrfs-filesystem-detail/btrfs-filesystem-detail.routing';
import { MatButtonModule } from '@angular/material/button';
import { MatDividerModule } from '@angular/material/divider';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSortModule } from '@angular/material/sort';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { NgApexchartsModule } from 'ng-apexcharts';
import { TreoCardModule } from '@treo/components/card';

@NgModule({
    imports: [
        RouterModule.forChild(btrfsFilesystemDetailRoutes),
        MatButtonModule,
        MatDividerModule,
        MatTooltipModule,
        MatIconModule,
        MatMenuModule,
        MatProgressBarModule,
        MatSortModule,
        MatTableModule,
        NgApexchartsModule,
        TreoCardModule,
        SharedModule,
        BtrfsFilesystemDetailComponent,
    ],
})
export class BtrfsFilesystemDetailModule {}
