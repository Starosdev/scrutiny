import { AsyncPipe } from '@angular/common';
import { Component, ViewEncapsulation, inject, ChangeDetectionStrategy } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { DashboardSettingsComponent } from 'app/layout/common/dashboard-settings/dashboard-settings.component';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { MatButton } from '@angular/material/button';
import { MatIcon } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

@Component({
    selector: 'mobile-settings',
    templateUrl: './mobile-settings.component.html',
    styleUrls: ['./mobile-settings.component.scss'],
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [AsyncPipe, MatButton, MatIcon, RouterLink],
})
export class MobileSettingsComponent {
    private readonly dialog = inject(MatDialog);

    readonly config$ = inject(ScrutinyConfigService).config$;

    openSettings(): void {
        this.dialog.open(DashboardSettingsComponent, {
            width: '100vw',
            maxWidth: '100vw',
            height: '100vh',
            panelClass: 'mobile-settings-dialog',
        });
    }
}
