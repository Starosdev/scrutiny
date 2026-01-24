import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { DateAdapter, MAT_DATE_FORMATS, MAT_DATE_LOCALE } from '@angular/material/core';
import { DayjsDateAdapter } from 'app/core/date-adapters/dayjs-date-adapter';
import { TreoDateRangeComponent } from '@treo/components/date-range/date-range.component';

export const DAYJS_DATE_FORMATS = {
    parse: {
        dateInput: 'DD/MM/YYYY',
    },
    display: {
        dateInput: 'DD/MM/YYYY',
        monthYearLabel: 'MMMM YYYY',
        dateA11yLabel: 'LL',
        monthYearA11yLabel: 'MMMM YYYY',
    },
};

@NgModule({
    declarations: [
        TreoDateRangeComponent
    ],
    imports     : [
        CommonModule,
        ReactiveFormsModule,
        MatButtonModule,
        MatDatepickerModule,
        MatFormFieldModule,
        MatInputModule,
        MatIconModule
    ],
    providers: [
        { provide: DateAdapter, useClass: DayjsDateAdapter },
        { provide: MAT_DATE_FORMATS, useValue: DAYJS_DATE_FORMATS },
    ],
    exports     : [
        TreoDateRangeComponent
    ]
})
export class TreoDateRangeModule
{
}
