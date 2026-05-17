import { NgModule } from '@angular/core';
import { RouterModule } from '@angular/router';
import { SharedModule } from 'app/shared/shared.module';
import { EmptyLayoutComponent } from 'app/layout/layouts/empty/empty.component';

@NgModule({
    imports: [RouterModule, SharedModule, EmptyLayoutComponent],
    exports: [EmptyLayoutComponent],
})
export class EmptyLayoutModule {}
