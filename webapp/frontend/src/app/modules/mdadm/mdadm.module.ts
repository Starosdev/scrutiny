import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { NgApexchartsModule } from 'ng-apexcharts';
import { SharedModule } from 'app/shared/shared.module';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { MDADMComponent } from 'app/modules/mdadm/mdadm.component';
import { MDADMDetailComponent } from 'app/modules/mdadm/details/mdadm-detail.component';
import { mdadmRoutes } from 'app/modules/mdadm/mdadm.routing';

@NgModule({
    declarations: [
        MDADMComponent,
        MDADMDetailComponent
    ],
    imports: [
        RouterModule.forChild(mdadmRoutes),
        NgApexchartsModule,
        SharedModule,
        MatIconModule,
        MatButtonModule
    ]
})
export class MDADMModule {
}
