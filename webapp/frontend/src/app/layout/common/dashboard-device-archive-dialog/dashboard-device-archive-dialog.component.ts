import {Component, Inject} from '@angular/core';
import {MAT_DIALOG_DATA, MatDialogRef as MatDialogRef} from '@angular/material/dialog';
import {DashboardDeviceArchiveDialogService} from 'app/layout/common/dashboard-device-archive-dialog/dashboard-device-archive-dialog.service';

@Component({
    selector: 'app-dashboard-device-archive-dialog',
    templateUrl: './dashboard-device-archive-dialog.component.html',
    styleUrls: ['./dashboard-device-archive-dialog.component.scss'],
    standalone: false
})
export class DashboardDeviceArchiveDialogComponent {

    constructor(
        public dialogRef: MatDialogRef<DashboardDeviceArchiveDialogComponent>,
        @Inject(MAT_DIALOG_DATA) public data: {deviceId: string, title: string},
        private _archiveService: DashboardDeviceArchiveDialogService,
    ) {
    }

  onArchiveClick(): void {
      this._archiveService.archiveDevice(this.data.deviceId)
          .subscribe((data) => {
              this.dialogRef.close(data);
          });

  }
}
