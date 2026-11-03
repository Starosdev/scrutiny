import { formatDate } from '@angular/common';
import { TemperatureUnit, TimeFormat } from 'app/core/config/app.config';
import { TemperaturePipe } from 'app/shared/temperature.pipe';
import { angularDateFormat } from 'app/shared/time-format.utils';

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
 * isEstimated reports whether a slot was interpolated by alignTemperatureChartSeries. It is a
 * callback rather than a captured array because _updateTemperatureChartSeries replaces the
 * series without rebuilding this closure, so a snapshot would go stale on every refresh.
 */
export function createTemperatureChartTooltip(
    document: Document,
    temperatureUnit: TemperatureUnit,
    timeFormat: TimeFormat,
    isEstimated: (seriesIndex: number, dataPointIndex: number) => boolean = () => false
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
            // formatTemperature formats only, it never converts: the prefix is added to its output
            // so an interpolated slot is not presented as a reading the drive sent.
            const formatted = String(TemperaturePipe.formatTemperature(series[seriesIndex]?.[dataPointIndex], temperatureUnit, true));
            const estimated = isEstimated(seriesIndex, dataPointIndex);
            value.textContent = estimated ? `~${formatted}` : formatted;
            if (estimated) {
                value.title = 'Estimated between readings';
            }
            valueGroup.appendChild(value);

            text.appendChild(valueGroup);
            row.appendChild(text);
            content.appendChild(row);
        });

        return content;
    };
}
