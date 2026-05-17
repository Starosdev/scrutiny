import { Component, inject } from '@angular/core';
import { MAT_DIALOG_DATA, MatDialogRef as MatDialogRef } from '@angular/material/dialog';
import { DashboardDeviceDeleteDialogService } from 'app/layout/common/dashboard-device-delete-dialog/dashboard-device-delete-dialog.service';

@Component({
    selector: 'app-dashboard-device-delete-dialog',
    templateUrl: './dashboard-device-delete-dialog.component.html',
    styleUrls: ['./dashboard-device-delete-dialog.component.scss'],
    standalone: false,
})
export class DashboardDeviceDeleteDialogComponent {
    dialogRef = inject<MatDialogRef<DashboardDeviceDeleteDialogComponent>>(MatDialogRef);
    data = inject<{
        deviceId: string;
        title: string;
    }>(MAT_DIALOG_DATA);
    private _deleteService = inject(DashboardDeviceDeleteDialogService);

    onDeleteClick(): void {
        this._deleteService.deleteDevice(this.data.deviceId).subscribe((data) => {
            this.dialogRef.close(data);
        });
    }
}
