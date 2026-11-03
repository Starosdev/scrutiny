export interface TemperatureChartPoint {
    x: Date;
    y: number | null;
    /** True when y was interpolated by alignTemperatureChartSeries rather than reported by the drive. */
    estimated?: boolean;
}

export interface TemperatureChartSeries {
    name: string;
    data: TemperatureChartPoint[];
}

const MAX_SNAP_TOLERANCE_MS = 15 * 60 * 1000;

/**
 * A gap wider than this many of a series' own median sampling intervals is a real
 * outage and stays null; anything narrower is a padding artifact of alignment. Below
 * two deliberately, so a single missed collector run, which leaves a gap of two
 * intervals, stays a gap at every cadence rather than only at long ones. The
 * remaining half interval is headroom for collector jitter.
 */
const MAX_FILL_INTERVALS = 1.5;

/**
 * Padding is filled only for a series with at least this many of its own intervals.
 * One interval cannot distinguish a cadence from an outage: a series holding two
 * readings a day apart would otherwise be drawn as a straight line across the whole
 * day view, which is the fabrication this bounding exists to prevent.
 */
const MIN_INTERVALS_FOR_CADENCE = 2;

interface IndexedTemperaturePoint {
    seriesIndex: number;
    timestamp: number;
    value: number | null;
}

interface TemperatureTimestampCluster {
    timestamps: number[];
    valuesBySeries: Map<number, number | null>;
}

function median(values: number[]): number {
    const sorted = [...values].sort((left, right) => left - right);
    return sorted[Math.floor((sorted.length - 1) / 2)];
}

/** Gaps between one series' own consecutive readings, in milliseconds. */
function seriesIntervals(entry: TemperatureChartSeries): number[] {
    const timestamps = entry.data
        .map((point) => point.x.getTime())
        .filter(Number.isFinite)
        .sort((left, right) => left - right);

    const intervals: number[] = [];
    for (let index = 1; index < timestamps.length; index++) {
        const interval = timestamps[index] - timestamps[index - 1];
        if (interval > 0) {
            intervals.push(interval);
        }
    }
    return intervals;
}

function snapTolerance(series: TemperatureChartSeries[]): number {
    const intervals = series.flatMap(seriesIntervals);

    return intervals.length > 0 ? Math.min(median(intervals) / 2, MAX_SNAP_TOLERANCE_MS) : 0;
}

function hasReading(point: TemperatureChartPoint): boolean {
    // Guard like the temperature pipe: a valid 0 degree reading is a reading.
    return point.y !== null && Number.isFinite(point.y);
}

/**
 * Alignment pads every series onto the shared index, so a series sampled less often
 * than its neighbours is null in most slots: ApexCharts draws no line through null
 * and the shared tooltip reports no value (#1076). ApexCharts 8 has no option to
 * connect a line across null, so the padded slots have to carry a value; interpolate
 * them linearly between the series' own readings, and only across gaps within
 * MAX_FILL_INTERVALS of this series' own median sampling interval. Wider gaps are a
 * real outage and stay visible as gaps, as do the slots before its first and after
 * its last reading.
 *
 * An interpolated slot reports a temperature the drive never sent, which is why the
 * bound is narrow. It is not the value the rendered line shows either: the stroke
 * curve is a user setting and defaults to 'smooth', so the drawn curve between two
 * readings is a spline rather than this straight segment.
 */
function interpolatePaddedReadings(points: TemperatureChartPoint[], maxGapMs: number): TemperatureChartPoint[] {
    if (maxGapMs <= 0) {
        return points;
    }

    const filled = [...points];
    let previous = -1;

    for (let index = 0; index < filled.length; index++) {
        if (!hasReading(filled[index])) {
            continue;
        }

        const gapStart = previous;
        previous = index;

        if (gapStart < 0 || index - gapStart < 2) {
            continue;
        }

        const startTime = filled[gapStart].x.getTime();
        const endTime = filled[index].x.getTime();
        if (endTime - startTime > maxGapMs) {
            continue;
        }

        const startValue = filled[gapStart].y as number;
        const endValue = filled[index].y as number;
        const span = endTime - startTime;

        for (let padded = gapStart + 1; padded < index; padded++) {
            const progress = span > 0 ? (filled[padded].x.getTime() - startTime) / span : 0;
            filled[padded] = { x: filled[padded].x, y: startValue + (endValue - startValue) * progress, estimated: true };
        }
    }

    return filled;
}

/**
 * ApexCharts shared datetime tooltips resolve every series with one data-point index.
 * Irregular arrays can therefore show a value and marker from the wrong timestamp.
 * Keep series indexes aligned by clustering nearby collection times, padding absent
 * readings with null, and limiting nearest-X snapping to 15 minutes. Do not restore
 * the old tooltipUtil monkey patch: it claimed overlap without aligning the indexes.
 */
export function alignTemperatureChartSeries(series: TemperatureChartSeries[]): TemperatureChartSeries[] {
    const points: IndexedTemperaturePoint[] = [];

    for (let seriesIndex = 0; seriesIndex < series.length; seriesIndex++) {
        const entry = series[seriesIndex];
        for (const point of entry.data) {
            const timestamp = point.x.getTime();
            if (Number.isFinite(timestamp)) {
                points.push({ seriesIndex, timestamp, value: point.y });
            }
        }
    }

    points.sort((left, right) => left.timestamp - right.timestamp || left.seriesIndex - right.seriesIndex);

    const tolerance = snapTolerance(series);
    const clusters: TemperatureTimestampCluster[] = [];

    for (const point of points) {
        const cluster = clusters[clusters.length - 1];
        const clusterCenter = cluster ? cluster.timestamps.reduce((sum, timestamp) => sum + timestamp, 0) / cluster.timestamps.length : 0;
        const canJoinCluster = cluster && !cluster.valuesBySeries.has(point.seriesIndex) && Math.abs(point.timestamp - clusterCenter) <= tolerance;

        if (canJoinCluster) {
            cluster.timestamps.push(point.timestamp);
            cluster.valuesBySeries.set(point.seriesIndex, point.value);
        } else {
            clusters.push({
                timestamps: [point.timestamp],
                valuesBySeries: new Map([[point.seriesIndex, point.value]]),
            });
        }
    }

    return series.map((entry, seriesIndex) => {
        const aligned = clusters.map((cluster) => ({
            x: new Date(median(cluster.timestamps)),
            y: cluster.valuesBySeries.get(seriesIndex) ?? null,
        }));

        const intervals = seriesIntervals(entry);
        const maxGapMs = intervals.length >= MIN_INTERVALS_FOR_CADENCE ? median(intervals) * MAX_FILL_INTERVALS : 0;

        return {
            name: entry.name,
            data: interpolatePaddedReadings(aligned, maxGapMs),
        };
    });
}

/**
 * What a slot of an aligned series actually represents.
 *
 * `stale` carries the series' nearest real reading and the time it was taken. Padding
 * outside the interpolation bound keeps a null y, so the line correctly shows a gap, but
 * the shared tooltip can still answer with a true statement about a real measurement
 * instead of "--". That is the only honest answer available for a series whose readings
 * are too far apart to interpolate, which on the default `0 0 * * *` collector schedule
 * is every drive without an SCT temperature history table.
 */
export type TemperatureSlotProvenance = { kind: 'measured' } | { kind: 'estimated' } | { kind: 'stale'; value: number; at: Date };

const MEASURED: TemperatureSlotProvenance = { kind: 'measured' };

/**
 * Classify every slot of every aligned series. The shared tooltip renders by series index
 * and data-point index, so that is the shape returned.
 */
export function temperatureSlotProvenance(series: TemperatureChartSeries[]): TemperatureSlotProvenance[][] {
    return series.map((entry) => {
        const readings = entry.data.filter((point) => hasReading(point) && point.estimated !== true);

        if (readings.length === 0) {
            return entry.data.map(() => MEASURED);
        }

        // Slots and readings are both in time order, so one forward pointer finds the
        // nearest reading for every slot in a single pass. The comparison below is strict,
        // so a slot exactly between two readings keeps the earlier one: the honest answer
        // is the last measurement taken, never one taken after the hovered time.
        let next = 0;

        return entry.data.map((point) => {
            if (point.estimated === true) {
                return { kind: 'estimated' } as TemperatureSlotProvenance;
            }
            if (hasReading(point)) {
                return MEASURED;
            }

            const slotTime = point.x.getTime();
            while (next + 1 < readings.length && Math.abs(readings[next + 1].x.getTime() - slotTime) < Math.abs(readings[next].x.getTime() - slotTime)) {
                next++;
            }

            const nearest = readings[next];
            return { kind: 'stale', value: nearest.y as number, at: nearest.x } as TemperatureSlotProvenance;
        });
    });
}
