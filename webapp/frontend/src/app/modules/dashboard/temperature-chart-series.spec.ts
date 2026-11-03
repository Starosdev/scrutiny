import { alignTemperatureChartSeries } from './temperature-chart-series';

describe('alignTemperatureChartSeries', () => {
    it('aligns irregular series to one timestamp index without moving readings', () => {
        const midnight = new Date('2026-08-01T00:00:00Z');
        const oneAm = new Date('2026-08-01T01:00:00Z');
        const twoAm = new Date('2026-08-01T02:00:00Z');

        const result = alignTemperatureChartSeries([
            {
                name: 'Steady drive',
                data: [
                    { x: midnight, y: 40 },
                    { x: oneAm, y: 45 },
                    { x: twoAm, y: 40 },
                ],
            },
            {
                name: 'Spike drive',
                data: [
                    { x: midnight, y: 35 },
                    { x: twoAm, y: 70 },
                ],
            },
        ]);

        expect(result.map((series) => series.data.map((point) => point.x.toISOString()))).toEqual([
            [midnight, oneAm, twoAm].map((date) => date.toISOString()),
            [midnight, oneAm, twoAm].map((date) => date.toISOString()),
        ]);
        expect(result[0].data.map((point) => point.y)).toEqual([40, 45, 40]);
        expect(result[1].data.map((point) => point.y)).toEqual([35, null, 70]);
    });

    it('sorts timestamps and preserves empty visible series', () => {
        const earlier = new Date('2026-08-01T00:00:00Z');
        const later = new Date('2026-08-01T01:00:00Z');

        const result = alignTemperatureChartSeries([
            {
                name: 'Out of order drive',
                data: [
                    { x: later, y: 50 },
                    { x: earlier, y: 40 },
                ],
            },
            { name: 'No readings drive', data: [] },
        ]);

        expect(result[0].data.map((point) => point.y)).toEqual([40, 50]);
        expect(result[1].data.map((point) => point.y)).toEqual([null, null]);
    });

    it('snaps nearby raw collection times to one shared hover index', () => {
        const timestamp = (minute: number) => new Date(Date.UTC(2026, 7, 1, 0, minute));

        const result = alignTemperatureChartSeries([
            {
                name: 'First collector',
                data: [
                    { x: timestamp(0), y: 40 },
                    { x: timestamp(10), y: 41 },
                    { x: timestamp(20), y: 42 },
                ],
            },
            {
                name: 'Second collector',
                data: [
                    { x: timestamp(1), y: 35 },
                    { x: timestamp(11), y: 36 },
                    { x: timestamp(21), y: 70 },
                ],
            },
        ]);

        expect(result[0].data.map((point) => point.x.toISOString())).toEqual([timestamp(0), timestamp(10), timestamp(20)].map((date) => date.toISOString()));
        expect(result[0].data.map((point) => point.y)).toEqual([40, 41, 42]);
        expect(result[1].data.map((point) => point.y)).toEqual([35, 36, 70]);
    });

    it('does not snap sparse readings that are hours apart', () => {
        const timestamp = (hour: number) => new Date(Date.UTC(2026, 7, 1, hour));

        const result = alignTemperatureChartSeries([
            {
                name: 'Sparse drive',
                data: [
                    { x: timestamp(0), y: 40 },
                    { x: timestamp(24), y: 42 },
                ],
            },
            {
                name: 'Other sparse drive',
                data: [{ x: timestamp(12), y: 50 }],
            },
        ]);

        expect(result[0].data.map((point) => point.y)).toEqual([40, null, 42]);
        expect(result[1].data.map((point) => point.y)).toEqual([null, 50, null]);
    });

    it('draws a continuous line for a series sampled less often than its neighbour', () => {
        // #1076: SATA drives contribute an SCT history table (one point per minute) and
        // dominate the shared snap tolerance, so every NVMe reading landed in its own
        // cluster and was null in all the others.
        const minutes = (count: number, step: number) => Array.from({ length: Math.floor(count / step) + 1 }, (_, index) => index * step);
        const series = (name: string, step: number, temp: number) => ({
            name,
            data: minutes(120, step).map((minute) => ({ x: new Date(Date.UTC(2026, 7, 1, 0, minute)), y: temp })),
        });

        const result = alignTemperatureChartSeries([series('SCT drive', 1, 40), series('NVMe drive', 15, 33)]);

        expect(result[1].data.length).toBe(121);
        expect(result[1].data.filter((point) => point.y === null).length).toBe(0);
        expect(result[1].data.every((point) => point.y === 33)).toBeTrue();
    });

    it('interpolates padded readings linearly between the series own readings', () => {
        const timestamp = (minute: number) => new Date(Date.UTC(2026, 7, 1, 0, minute));

        const result = alignTemperatureChartSeries([
            { name: 'Dense drive', data: Array.from({ length: 21 }, (_, minute) => ({ x: timestamp(minute), y: 40 })) },
            {
                name: 'Sparse drive',
                data: [
                    { x: timestamp(0), y: 30 },
                    { x: timestamp(10), y: 34 },
                    { x: timestamp(20), y: 38 },
                ],
            },
        ]);

        expect(result[1].data[0].y).toBe(30);
        expect(result[1].data[10].y).toBe(34);
        expect(result[1].data[5].y).toBe(32);
        expect(result[1].data[15].y).toBe(36);
    });

    it('keeps a collector outage visible as a gap', () => {
        const timestamp = (minute: number) => new Date(Date.UTC(2026, 7, 1, 0, minute));

        const result = alignTemperatureChartSeries([
            { name: 'Dense drive', data: Array.from({ length: 271 }, (_, minute) => ({ x: timestamp(minute), y: 40 })) },
            {
                name: 'Interrupted drive',
                data: [0, 15, 30, 45, 240, 255, 270].map((minute) => ({ x: timestamp(minute), y: 33 })),
            },
        ]);

        expect(result[1].data[45].y).toBe(33);
        expect(result[1].data[240].y).toBe(33);
        expect(result[1].data.slice(46, 240).every((point) => point.y === null)).toBeTrue();
    });

    it('does not pad before the first or after the last reading of a series', () => {
        const timestamp = (minute: number) => new Date(Date.UTC(2026, 7, 1, 0, minute));

        const result = alignTemperatureChartSeries([
            { name: 'Dense drive', data: Array.from({ length: 31 }, (_, minute) => ({ x: timestamp(minute), y: 40 })) },
            {
                name: 'Late drive',
                data: [10, 15, 20].map((minute) => ({ x: timestamp(minute), y: 33 })),
            },
        ]);

        expect(result[1].data.slice(0, 10).every((point) => point.y === null)).toBeTrue();
        expect(result[1].data.slice(10, 21).every((point) => point.y === 33)).toBeTrue();
        expect(result[1].data.slice(21).every((point) => point.y === null)).toBeTrue();
    });

    it('treats a zero degree reading as a reading when padding', () => {
        const timestamp = (minute: number) => new Date(Date.UTC(2026, 7, 1, 0, minute));

        const result = alignTemperatureChartSeries([
            { name: 'Dense drive', data: Array.from({ length: 31 }, (_, minute) => ({ x: timestamp(minute), y: 10 })) },
            {
                name: 'Freezing drive',
                data: [0, 15, 30].map((minute) => ({ x: timestamp(minute), y: 0 })),
            },
        ]);

        expect(result[1].data[15].y).toBe(0);
        expect(result[1].data[7].y).toBe(0);
        expect(result[1].data.some((point) => point.y === null)).toBeFalse();
    });

    it('fills a gap of exactly MAX_FILL_INTERVALS and leaves a wider one alone', () => {
        const timestamp = (minute: number) => new Date(Date.UTC(2026, 7, 1, 0, minute));
        const dense = (count: number) => Array.from({ length: count }, (_, minute) => ({ x: timestamp(minute), y: 40 }));
        const sparse = (last: number) => [0, 10, 20, last].map((minute) => ({ x: timestamp(minute), y: 33 }));

        // Median interval 10 minutes, so the bound is 15 minutes.
        const atBound = alignTemperatureChartSeries([
            { name: 'Dense drive', data: dense(36) },
            { name: 'Sparse drive', data: sparse(35) },
        ]);
        expect(atBound[1].data.slice(20, 36).every((point) => point.y === 33)).toBeTrue();

        const pastBound = alignTemperatureChartSeries([
            { name: 'Dense drive', data: dense(37) },
            { name: 'Sparse drive', data: sparse(36) },
        ]);
        expect(pastBound[1].data[20].y).toBe(33);
        expect(pastBound[1].data.slice(21, 36).every((point) => point.y === null)).toBeTrue();
        expect(pastBound[1].data[36].y).toBe(33);
    });

    it('keeps a single missed collector run visible as a gap at a long cadence', () => {
        // The default collector cron is daily, and a long interval must not buy a
        // wider fill: one missed run is a gap at every cadence.
        const hours = (hour: number) => new Date(Date.UTC(2026, 7, 1, hour));

        const result = alignTemperatureChartSeries([
            { name: 'Dense drive', data: Array.from({ length: 25 }, (_, hour) => ({ x: hours(hour), y: 40 })) },
            { name: 'Eight hourly drive', data: [0, 8, 24].map((hour) => ({ x: hours(hour), y: 33 })) },
        ]);

        expect(result[1].data[8].y).toBe(33);
        expect(result[1].data.slice(9, 24).every((point) => point.y === null)).toBeTrue();
    });

    it('interpolates on elapsed time, not on the number of padded slots', () => {
        // The shared index is irregular, so a slot's position in the array says
        // nothing about where it falls between two readings.
        const timestamp = (minute: number) => new Date(Date.UTC(2026, 7, 1, 0, minute));

        const result = alignTemperatureChartSeries([
            { name: 'Irregular dense drive', data: [0, 2, 6, 8, 10, 14, 16].map((minute) => ({ x: timestamp(minute), y: 40 })) },
            {
                name: 'Sparse drive',
                data: [
                    { x: timestamp(0), y: 30 },
                    { x: timestamp(8), y: 38 },
                    { x: timestamp(16), y: 46 },
                ],
            },
        ]);

        expect(result[1].data.map((point) => point.x.getTime())).toEqual([0, 2, 6, 8, 10, 14, 16].map((minute) => timestamp(minute).getTime()));
        expect(result[1].data[1].y).toBe(32);
        expect(result[1].data[2].y).toBe(36);
        expect(result[1].data[4].y).toBe(40);
        expect(result[1].data[5].y).toBe(44);
    });
});
