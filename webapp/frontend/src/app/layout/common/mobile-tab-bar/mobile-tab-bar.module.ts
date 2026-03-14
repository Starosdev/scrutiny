import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { MatIconModule } from '@angular/material/icon';
import { SharedModule } from 'app/shared/shared.module';
import { MobileTabBarComponent } from './mobile-tab-bar.component';

@NgModule({
    declarations: [
        MobileTabBarComponent
    ],
    exports: [
        MobileTabBarComponent
    ],
    imports: [
        RouterModule,
        MatIconModule,
        SharedModule
    ]
})
export class MobileTabBarModule {}
