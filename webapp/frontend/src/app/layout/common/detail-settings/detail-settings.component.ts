import { Component, inject } from '@angular/core';
import { MAT_DIALOG_DATA } from '@angular/material/dialog';

@Component({
    selector: 'app-detail-settings',
    templateUrl: './detail-settings.component.html',
    styleUrls: ['./detail-settings.component.scss'],
    standalone: false,
})
export class DetailSettingsComponent {
    data = inject<{
        curMuted: boolean;
        curLabel: string;
        curMissedPingTimeoutOverride: number;
        globalMissedPingTimeout: number;
    }>(MAT_DIALOG_DATA);

    muted: boolean;
    label: string;
    missedPingTimeoutOverride: number;

    constructor() {
        const data = this.data;

        this.muted = data.curMuted;
        this.label = data.curLabel || '';
        this.missedPingTimeoutOverride = data.curMissedPingTimeoutOverride || 0;
    }
}
