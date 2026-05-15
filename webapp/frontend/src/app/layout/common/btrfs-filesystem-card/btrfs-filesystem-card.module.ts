import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { CommonModule } from '@angular/common';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatDialogModule } from '@angular/material/dialog';
import { SharedModule } from 'app/shared/shared.module';
import { BtrfsFilesystemCardComponent } from './btrfs-filesystem-card.component';

@NgModule({
    declarations: [BtrfsFilesystemCardComponent],
    imports: [CommonModule, RouterModule, MatButtonModule, MatIconModule, MatMenuModule, MatTooltipModule, SharedModule, MatDialogModule],
    exports: [BtrfsFilesystemCardComponent],
})
export class BtrfsFilesystemCardModule {}
