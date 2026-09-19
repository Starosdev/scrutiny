import { Component, inject, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MAT_DIALOG_DATA, MatDialogActions, MatDialogClose, MatDialogContent, MatDialogRef, MatDialogTitle } from '@angular/material/dialog';
import { MatButton } from '@angular/material/button';
import { MatFormField, MatLabel } from '@angular/material/form-field';
import { MatInput } from '@angular/material/input';

@Component({
    selector: 'host-purge-dialog',
    templateUrl: './host-purge-dialog.component.html',
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [FormsModule, MatDialogTitle, MatDialogContent, MatDialogActions, MatDialogClose, MatButton, MatFormField, MatLabel, MatInput],
})
export class HostPurgeDialogComponent {
    readonly data = inject<{ hostIds: string[] }>(MAT_DIALOG_DATA);
    private readonly dialogRef = inject<MatDialogRef<HostPurgeDialogComponent>>(MatDialogRef);

    confirmation = '';

    confirm(): void {
        if (this.confirmation === 'PURGE') {
            this.dialogRef.close(true);
        }
    }
}
