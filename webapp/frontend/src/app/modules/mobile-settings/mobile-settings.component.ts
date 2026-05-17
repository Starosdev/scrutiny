import { Component, ViewEncapsulation, inject } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { DashboardSettingsComponent } from 'app/layout/common/dashboard-settings/dashboard-settings.component';
import { versionInfo } from 'environments/versions';
import { MatButton } from '@angular/material/button';
import { MatIcon } from '@angular/material/icon';

@Component({
    selector: 'mobile-settings',
    templateUrl: './mobile-settings.component.html',
    styleUrls: ['./mobile-settings.component.scss'],
    encapsulation: ViewEncapsulation.None,
    imports: [MatButton, MatIcon],
})
export class MobileSettingsComponent {
    private readonly dialog = inject(MatDialog);

    appVersion: string = versionInfo.version;

    openSettings(): void {
        this.dialog.open(DashboardSettingsComponent, {
            width: '100vw',
            maxWidth: '100vw',
            height: '100vh',
            panelClass: 'mobile-settings-dialog',
        });
    }
}
