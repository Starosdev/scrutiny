import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TreoCardComponent } from '@treo/components/card/card.component';

@NgModule({
    imports: [CommonModule, TreoCardComponent],
    exports: [TreoCardComponent],
})
export class TreoCardModule {}
