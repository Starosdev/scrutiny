import { NgModule } from '@angular/core';
import { TreoScrollbarDirective } from '@treo/directives/scrollbar/scrollbar.directive';

@NgModule({
    imports: [TreoScrollbarDirective],
    exports: [TreoScrollbarDirective],
})
export class TreoScrollbarModule {}
