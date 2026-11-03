import { formatDate } from '@angular/common';
import { TemperatureUnit, TimeFormat } from 'app/core/config/app.config';
import { TemperaturePipe } from 'app/shared/temperature.pipe';
import { angularDateFormat } from 'app/shared/time-format.utils';
import { TemperatureSlotProvenance } from './temperature-chart-series';

interface TemperatureTooltipChartContext {
    globals: {
        ancillaryCollapsedSeriesIndices?: number[];
        collapsedSeriesIndices?: number[];
        colors: string[];
        seriesNames: string[];
        seriesX: number[][];
    };
}

interface TemperatureTooltipOptions {
    dataPointIndex: number;
    series: (number | null)[][];
    w: TemperatureTooltipChartContext;
}

/**
 * provenanceFor reports what a slot represents: a reading, a value interpolated by
 * alignTemperatureChartSeries, or a null slot that can still name the series' nearest real
 * reading. It is a callback rather than a captured array because _updateTemperatureChartSeries
 * replaces the series without rebuilding this closure, so a snapshot would go stale on every
 * refresh.
 */
export function createTemperatureChartTooltip(
    document: Document,
    temperatureUnit: TemperatureUnit,
    timeFormat: TimeFormat,
    provenanceFor: (seriesIndex: number, dataPointIndex: number) => TemperatureSlotProvenance | undefined = () => undefined
): (options: TemperatureTooltipOptions) => Element {
    return ({ dataPointIndex, series, w }) => {
        const content = document.createElement('div');
        const hiddenSeries = new Set([...(w.globals.collapsedSeriesIndices ?? []), ...(w.globals.ancillaryCollapsedSeriesIndices ?? [])]);
        const timestamp = w.globals.seriesX.find((timestamps) => Number.isFinite(timestamps?.[dataPointIndex]))?.[dataPointIndex];

        if (Number.isFinite(timestamp)) {
            const title = document.createElement('div');
            title.className = 'apexcharts-tooltip-title';
            title.textContent = formatDate(timestamp, angularDateFormat('MMM dd, yyyy', timeFormat, true), 'en-US');
            content.appendChild(title);
        }

        w.globals.seriesNames.forEach((name, seriesIndex) => {
            if (hiddenSeries.has(seriesIndex)) {
                return;
            }

            const row = document.createElement('div');
            row.className = 'apexcharts-tooltip-series-group apexcharts-active';
            row.style.display = 'flex';

            const marker = document.createElement('span');
            marker.className = 'apexcharts-tooltip-marker';
            marker.style.backgroundColor = w.globals.colors[seriesIndex];
            marker.style.borderRadius = '50%';
            row.appendChild(marker);

            const text = document.createElement('div');
            text.className = 'apexcharts-tooltip-text';

            const valueGroup = document.createElement('div');
            valueGroup.className = 'apexcharts-tooltip-y-group';

            const label = document.createElement('span');
            label.className = 'apexcharts-tooltip-text-y-label';
            label.textContent = `${name}: `;
            valueGroup.appendChild(label);

            const value = document.createElement('span');
            value.className = 'apexcharts-tooltip-text-y-value';
            // Series reaching alignment are already in the display unit, so formatTemperature
            // only formats here and never converts. Markers are added to its output.
            const provenance = provenanceFor(seriesIndex, dataPointIndex);

            if (provenance?.kind === 'stale') {
                const readingTime = formatDate(provenance.at, angularDateFormat('', timeFormat), 'en-US');
                value.textContent = `${TemperaturePipe.formatTemperature(provenance.value, temperatureUnit, true)} @ ${readingTime}`;
                value.title = `No reading at this time. Nearest reading was taken at ${readingTime}.`;
            } else {
                const formatted = String(TemperaturePipe.formatTemperature(series[seriesIndex]?.[dataPointIndex], temperatureUnit, true));
                const estimated = provenance?.kind === 'estimated';
                value.textContent = estimated ? `~${formatted}` : formatted;
                if (estimated) {
                    value.title = 'Estimated between readings';
                }
            }
            valueGroup.appendChild(value);

            text.appendChild(valueGroup);
            row.appendChild(text);
            content.appendChild(row);
        });

        return content;
    };
}
