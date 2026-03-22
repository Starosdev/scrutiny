import { Component, Inject } from '@angular/core';
import { MAT_DIALOG_DATA } from '@angular/material/dialog';

@Component({
    selector: 'app-detail-settings',
    templateUrl: './detail-settings.component.html',
    styleUrls: ['./detail-settings.component.scss'],
    standalone: false
})
export class DetailSettingsComponent {

  muted: boolean;
  label: string;
  missedPingTimeoutOverride: number;

  constructor(
      @Inject(MAT_DIALOG_DATA) public data: {
          curMuted: boolean,
          curLabel: string,
          curMissedPingTimeoutOverride: number,
          globalMissedPingTimeout: number
      }
  ) {
      this.muted = data.curMuted;
      this.label = data.curLabel || '';
      this.missedPingTimeoutOverride = data.curMissedPingTimeoutOverride || 0;
  }
}
