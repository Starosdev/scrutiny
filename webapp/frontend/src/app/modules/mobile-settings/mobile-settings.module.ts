import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { SharedModule } from 'app/shared/shared.module';
import { DashboardSettingsModule } from 'app/layout/common/dashboard-settings/dashboard-settings.module';
import { MobileSettingsComponent } from './mobile-settings.component';
import { mobileSettingsRoutes } from './mobile-settings.routing';

@NgModule({
    declarations: [
        MobileSettingsComponent
    ],
    imports: [
        RouterModule.forChild(mobileSettingsRoutes),
        MatButtonModule,
        MatIconModule,
        SharedModule,
        DashboardSettingsModule
    ]
})
export class MobileSettingsModule {}
