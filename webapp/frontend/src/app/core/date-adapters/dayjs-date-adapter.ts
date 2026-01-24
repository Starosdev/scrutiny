import { Injectable } from '@angular/core';
import { NativeDateAdapter } from '@angular/material/core';
import dayjs from 'dayjs';

@Injectable()
export class DayjsDateAdapter extends NativeDateAdapter {
    parse(value: any): Date | null {
        if (value && typeof value === 'string') {
            const dayjsValue = dayjs(value);
            return dayjsValue.isValid() ? dayjsValue.toDate() : null;
        }
        return value ? dayjs(value).toDate() : null;
    }

    format(date: Date, displayFormat: string): string {
        return dayjs(date).format(displayFormat);
    }
}
