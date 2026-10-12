import { Pipe, PipeTransform } from '@angular/core';
import { MetricsStatusThreshold } from 'app/core/config/app.config';
import { DeviceTitlePipe } from 'app/shared/device-title.pipe';

@Pipe({ name: 'deviceSort' })
export class DeviceSortPipe implements PipeTransform {
    statusCompareFn(statusThreshold: MetricsStatusThreshold) {
        // Severity rank, higher is worse, so status_desc ("Failed First") lists failing drives first.
        // Like the dashboard badge, it applies the status threshold to device_status and treats a
        // forced failure as failed regardless of the threshold.
        // Mirrors summaryStatusValue in the backend summary page sort.
        return function (a: any, b: any): number {
            function severity(deviceSummary): number {
                if (!deviceSummary.smart) {
                    return 1; // unknown, matches the badge shown without SMART results
                }
                if (deviceSummary.device.has_forced_failure) {
                    return 5;
                }
                // tslint:disable-next-line:no-bitwise
                const status = deviceSummary.device.device_status & statusThreshold;
                return status === 0 ? 0 : status + 1; // passed 0, failed smart 2, scrutiny 3, both 4
            }

            return severity(a) - severity(b);
        };
    }

    titleCompareFn(dashboardDisplay: string) {
        return function (a: any, b: any) {
            const _dashboardDisplay = dashboardDisplay;
            const left = DeviceTitlePipe.deviceTitleForType(a.device, _dashboardDisplay) || DeviceTitlePipe.deviceTitleForType(a.device, 'name');
            const right = DeviceTitlePipe.deviceTitleForType(b.device, _dashboardDisplay) || DeviceTitlePipe.deviceTitleForType(b.device, 'name');

            if (left < right) return -1;

            if (left > right) return 1;

            return 0;
        };
    }

    ageCompareFn(a: any, b: any): number {
        const left = a.smart?.power_on_hours ?? 0;
        const right = b.smart?.power_on_hours ?? 0;

        return left - right;
    }

    capacityCompareFn(a: any, b: any): number {
        const left = a.device?.capacity || 0;
        const right = b.device?.capacity || 0;

        return left - right;
    }

    temperatureCompareFn(a: any, b: any): number {
        const left = a.smart?.temp ?? Number.MAX_SAFE_INTEGER;
        const right = b.smart?.temp ?? Number.MAX_SAFE_INTEGER;

        return left - right;
    }

    transform(
        deviceSummaries: Array<unknown>,
        sortBy = 'status',
        dashboardDisplay = 'name',
        statusThreshold: MetricsStatusThreshold = MetricsStatusThreshold.Both
    ): Array<unknown> {
        // Map legacy values to new format for backward compatibility
        const legacyMap: Record<string, string> = {
            status: 'status_desc',
            title: 'title_asc',
            age: 'age_asc',
        };
        const normalizedSort = legacyMap[sortBy] || sortBy;

        // Parse sort field and direction
        const isDesc = normalizedSort.endsWith('_desc');
        const sortField = normalizedSort.replace(/_(?:asc|desc)$/, '');
        const direction = isDesc ? -1 : 1;

        // Get compare function
        let compareFn: (a: any, b: any) => number;
        switch (sortField) {
            case 'status':
                compareFn = this.statusCompareFn(statusThreshold || MetricsStatusThreshold.Both);
                break;
            case 'title':
                compareFn = this.titleCompareFn(dashboardDisplay);
                break;
            case 'age':
                compareFn = this.ageCompareFn;
                break;
            case 'capacity':
                compareFn = this.capacityCompareFn;
                break;
            case 'temperature':
                compareFn = this.temperatureCompareFn;
                break;
            default:
                compareFn = this.statusCompareFn(statusThreshold || MetricsStatusThreshold.Both);
        }

        // Apply sort with direction
        deviceSummaries.sort((a, b) => direction * compareFn(a, b));

        return deviceSummaries;
    }
}
