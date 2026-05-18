import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatDialogModule } from '@angular/material/dialog';
import { MatButtonModule } from '@angular/material/button';
import { NgApexchartsModule } from 'ng-apexcharts';
import { AttributeHistoryDialogComponent } from './attribute-history-dialog.component';

@NgModule({
    imports: [CommonModule, MatDialogModule, MatButtonModule, NgApexchartsModule, AttributeHistoryDialogComponent],
    exports: [AttributeHistoryDialogComponent],
})
export class AttributeHistoryDialogModule {}
