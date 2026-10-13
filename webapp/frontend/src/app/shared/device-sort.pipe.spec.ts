import { MetricsStatusThreshold } from 'app/core/config/app.config';
import { DeviceSortPipe } from './device-sort.pipe';

describe('DeviceSortPipe', () => {
    const summary = (id: string, deviceStatus: number, forced = false, smart = true) => ({
        device: { device_id: id, device_status: deviceStatus, has_forced_failure: forced },
        smart: smart ? {} : undefined,
    });

    const summaries = () => [
        summary('passed', 0),
        summary('both', 3),
        summary('unknown', 0, false, false),
        summary('forced', 0, true),
        summary('smart', 1),
        summary('scrutiny', 2),
    ];

    const sortedIds = (sortBy: string, threshold?: MetricsStatusThreshold): string[] =>
        new DeviceSortPipe().transform(summaries(), sortBy, 'name', threshold).map((s: any) => s.device.device_id);

    const failedFirst = ['forced', 'both', 'scrutiny', 'smart', 'unknown', 'passed'];

    it('create an instance', () => {
        const pipe = new DeviceSortPipe();
        expect(pipe).toBeTruthy();
    });

    it('lists failed drives first for status_desc', () => {
        expect(sortedIds('status_desc')).toEqual(failedFirst);
    });

    it('lists failed drives first for the legacy status value', () => {
        expect(sortedIds('status')).toEqual(failedFirst);
    });

    it('lists passed drives first for status_asc', () => {
        expect(sortedIds('status_asc')).toEqual(['passed', 'unknown', 'smart', 'scrutiny', 'both', 'forced']);
    });

    it('ranks failures hidden by the SMART-only threshold with passed drives', () => {
        const ids = sortedIds('status_desc', MetricsStatusThreshold.Smart);
        expect(ids.slice(0, 4)).toEqual(['forced', 'both', 'smart', 'unknown']);
        expect(ids.slice(4).sort()).toEqual(['passed', 'scrutiny']);
    });

    it('ranks failures hidden by the Scrutiny-only threshold with passed drives', () => {
        const ids = sortedIds('status_desc', MetricsStatusThreshold.Scrutiny);
        expect(ids.slice(0, 4)).toEqual(['forced', 'both', 'scrutiny', 'unknown']);
        expect(ids.slice(4).sort()).toEqual(['passed', 'smart']);
    });
});
