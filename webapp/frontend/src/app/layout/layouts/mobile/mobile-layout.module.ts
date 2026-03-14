import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MobileTabBarModule } from 'app/layout/common/mobile-tab-bar/mobile-tab-bar.module';
import { ThemeToggleModule } from 'app/layout/common/theme-toggle/theme-toggle.module';
import { SharedModule } from 'app/shared/shared.module';
import { MobileLayoutComponent } from './mobile-layout.component';

@NgModule({
    declarations: [
        MobileLayoutComponent
    ],
    exports: [
        MobileLayoutComponent
    ],
    imports: [
        RouterModule,
        MatButtonModule,
        MatIconModule,
        MatTooltipModule,
        MobileTabBarModule,
        ThemeToggleModule,
        SharedModule
    ]
})
export class MobileLayoutModule {}
