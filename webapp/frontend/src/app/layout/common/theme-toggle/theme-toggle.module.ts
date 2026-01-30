import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ThemeToggleComponent } from 'app/layout/common/theme-toggle/theme-toggle.component';

@NgModule({
    declarations: [
        ThemeToggleComponent
    ],
    imports: [
        CommonModule,
        MatButtonModule,
        MatIconModule,
        MatTooltipModule
    ],
    exports: [
        ThemeToggleComponent
    ]
})
export class ThemeToggleModule {}
