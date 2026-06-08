import { Component, inject } from '@angular/core';
import { MAT_DIALOG_DATA, MatDialogRef as MatDialogRef, MatDialogTitle, MatDialogContent, MatDialogActions, MatDialogClose } from '@angular/material/dialog';
import { DashboardDeviceArchiveDialogService } from 'app/layout/common/dashboard-device-archive-dialog/dashboard-device-archive-dialog.service';
import { CdkScrollable } from '@angular/cdk/scrolling';
import { MatButton } from '@angular/material/button';
import { MatIcon } from '@angular/material/icon';

@Component({
    selector: 'app-dashboard-device-archive-dialog',
    templateUrl: './dashboard-device-archive-dialog.component.html',
    styleUrls: ['./dashboard-device-archive-dialog.component.scss'],
    imports: [MatDialogTitle, CdkScrollable, MatDialogContent, MatDialogActions, MatButton, MatDialogClose, MatIcon],
})
export class DashboardDeviceArchiveDialogComponent {
    dialogRef = inject<MatDialogRef<DashboardDeviceArchiveDialogComponent>>(MatDialogRef);
    data = inject<{
        deviceId: string;
        title: string;
    }>(MAT_DIALOG_DATA);
    private readonly _archiveService = inject(DashboardDeviceArchiveDialogService);

    onArchiveClick(): void {
        this._archiveService.archiveDevice(this.data.deviceId).subscribe((data) => {
            this.dialogRef.close(data);
        });
    }
}
