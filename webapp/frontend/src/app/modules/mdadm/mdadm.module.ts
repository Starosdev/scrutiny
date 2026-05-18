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
    imports: [RouterModule.forChild(mdadmRoutes), NgApexchartsModule, SharedModule, MatIconModule, MatButtonModule, MDADMComponent, MDADMDetailComponent],
})
export class MDADMModule {}
