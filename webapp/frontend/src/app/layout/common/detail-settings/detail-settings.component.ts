import { Component, inject } from '@angular/core';
import { MAT_DIALOG_DATA, MatDialogTitle, MatDialogContent, MatDialogActions, MatDialogClose } from '@angular/material/dialog';
import { CdkScrollable } from '@angular/cdk/scrolling';
import { ReactiveFormsModule, FormsModule } from '@angular/forms';
import { MatFormField, MatLabel, MatHint } from '@angular/material/form-field';
import { MatInput } from '@angular/material/input';
import { MatSelect } from '@angular/material/select';
import { MatOption } from '@angular/material/autocomplete';
import { MatButton } from '@angular/material/button';

@Component({
    selector: 'app-detail-settings',
    templateUrl: './detail-settings.component.html',
    styleUrls: ['./detail-settings.component.scss'],
    imports: [
        MatDialogTitle,
        CdkScrollable,
        MatDialogContent,
        ReactiveFormsModule,
        FormsModule,
        MatFormField,
        MatLabel,
        MatInput,
        MatHint,
        MatSelect,
        MatOption,
        MatDialogActions,
        MatButton,
        MatDialogClose,
    ],
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
