import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { MatIconModule } from '@angular/material/icon';
import { SharedModule } from 'app/shared/shared.module';
import { MobileHomeComponent } from './mobile-home.component';
import { mobileHomeRoutes } from './mobile-home.routing';

@NgModule({
    declarations: [
        MobileHomeComponent
    ],
    imports: [
        RouterModule.forChild(mobileHomeRoutes),
        MatIconModule,
        SharedModule
    ]
})
export class MobileHomeModule {}
