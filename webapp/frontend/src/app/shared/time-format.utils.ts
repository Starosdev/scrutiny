import {TimeFormat} from 'app/core/config/app.config';

// Angular date pipe / formatDate() format tokens
const ANGULAR_TIME_24 = 'HH:mm';
const ANGULAR_TIME_12 = 'h:mm a';
const ANGULAR_TIME_24_SEC = 'HH:mm:ss';
const ANGULAR_TIME_12_SEC = 'h:mm:ss a';

// ApexCharts (date-fns) format tokens
const APEX_TIME_24 = 'HH:mm';
const APEX_TIME_12 = 'hh:mm tt';
const APEX_TIME_24_SEC = 'HH:mm:ss';
const APEX_TIME_12_SEC = 'hh:mm:ss tt';

function is12(fmt: TimeFormat): boolean {
    return fmt === '12';
}

export function angularDateFormat(datepart: string, timeFormat: TimeFormat, includeSeconds = false): string {
    const time = is12(timeFormat)
        ? (includeSeconds ? ANGULAR_TIME_12_SEC : ANGULAR_TIME_12)
        : (includeSeconds ? ANGULAR_TIME_24_SEC : ANGULAR_TIME_24);
    return datepart ? `${datepart} ${time}` : time;
}

export function angularLongDateTime(timeFormat: TimeFormat): string {
    return angularDateFormat('MMMM dd, yyyy -', timeFormat);
}

export function angularShortDateTime(timeFormat: TimeFormat): string {
    return angularDateFormat('MMM dd, yyyy', timeFormat);
}

export function apexDateFormat(datepart: string, timeFormat: TimeFormat, includeSeconds = false): string {
    const time = is12(timeFormat)
        ? (includeSeconds ? APEX_TIME_12_SEC : APEX_TIME_12)
        : (includeSeconds ? APEX_TIME_24_SEC : APEX_TIME_24);
    return datepart ? `${datepart} ${time}` : time;
}

export function apexShortDateTime(timeFormat: TimeFormat, includeSeconds = false): string {
    return apexDateFormat('MMM dd, yyyy', timeFormat, includeSeconds);
}
