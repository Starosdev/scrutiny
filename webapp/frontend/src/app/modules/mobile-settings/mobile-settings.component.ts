import { Component, ViewEncapsulation } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { DashboardSettingsComponent } from 'app/layout/common/dashboard-settings/dashboard-settings.component';
import { versionInfo } from 'environments/versions';

@Component({
    selector: 'mobile-settings',
    templateUrl: './mobile-settings.component.html',
    styleUrls: ['./mobile-settings.component.scss'],
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class MobileSettingsComponent {
    appVersion: string = versionInfo.version;

    constructor(private dialog: MatDialog) {}

    openSettings(): void {
        this.dialog.open(DashboardSettingsComponent, {
            width: '100vw',
            maxWidth: '100vw',
            height: '100vh',
            panelClass: 'mobile-settings-dialog'
        });
    }
}
