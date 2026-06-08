import { Component, inject } from '@angular/core';
import { MAT_DIALOG_DATA, MatDialogRef as MatDialogRef, MatDialogTitle, MatDialogContent, MatDialogActions, MatDialogClose } from '@angular/material/dialog';
import { DashboardDeviceDeleteDialogService } from 'app/layout/common/dashboard-device-delete-dialog/dashboard-device-delete-dialog.service';
import { CdkScrollable } from '@angular/cdk/scrolling';
import { MatButton } from '@angular/material/button';
import { MatIcon } from '@angular/material/icon';

@Component({
    selector: 'app-dashboard-device-delete-dialog',
    templateUrl: './dashboard-device-delete-dialog.component.html',
    styleUrls: ['./dashboard-device-delete-dialog.component.scss'],
    imports: [MatDialogTitle, CdkScrollable, MatDialogContent, MatDialogActions, MatButton, MatDialogClose, MatIcon],
})
export class DashboardDeviceDeleteDialogComponent {
    dialogRef = inject<MatDialogRef<DashboardDeviceDeleteDialogComponent>>(MatDialogRef);
    data = inject<{
        deviceId: string;
        title: string;
    }>(MAT_DIALOG_DATA);
    private readonly _deleteService = inject(DashboardDeviceDeleteDialogService);

    onDeleteClick(): void {
        this._deleteService.deleteDevice(this.data.deviceId).subscribe((data) => {
            this.dialogRef.close(data);
        });
    }
}
