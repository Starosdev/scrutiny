import {Component, Inject} from '@angular/core';
import {MAT_DIALOG_DATA, MatDialogRef as MatDialogRef} from '@angular/material/dialog';
import {DashboardDeviceDeleteDialogService} from 'app/layout/common/dashboard-device-delete-dialog/dashboard-device-delete-dialog.service';

@Component({
    selector: 'app-dashboard-device-delete-dialog',
    templateUrl: './dashboard-device-delete-dialog.component.html',
    styleUrls: ['./dashboard-device-delete-dialog.component.scss'],
    standalone: false
})
export class DashboardDeviceDeleteDialogComponent {

    constructor(
        public dialogRef: MatDialogRef<DashboardDeviceDeleteDialogComponent>,
        @Inject(MAT_DIALOG_DATA) public data: {deviceId: string, title: string},
        private _deleteService: DashboardDeviceDeleteDialogService,
    ) {
    }

  onDeleteClick(): void {
      this._deleteService.deleteDevice(this.data.deviceId)
          .subscribe((data) => {
              this.dialogRef.close(data);
          });

  }
}
