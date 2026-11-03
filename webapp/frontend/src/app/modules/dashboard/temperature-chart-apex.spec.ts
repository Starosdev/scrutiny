import ApexCharts from 'apexcharts';
import { alignTemperatureChartSeries, estimatedTemperaturePoints } from './temperature-chart-series';
import { createTemperatureChartTooltip } from './temperature-chart-tooltip';

describe('dashboard temperature ApexCharts integration', () => {
    let chart: ApexCharts;
    let host: HTMLDivElement;

    beforeEach(() => {
        host = document.createElement('div');
        host.style.width = '700px';
        host.style.height = '320px';
        document.body.appendChild(host);
    });

    afterEach(() => {
        chart?.destroy();
        host.remove();
    });

    it('keeps the fixed data panel stable across aligned missing readings', async () => {
        const timestamp = (hour: number) => new Date(Date.UTC(2026, 7, 1, hour));
        const series = alignTemperatureChartSeries([
            {
                name: 'Steady drive',
                data: [
                    { x: timestamp(0), y: 40 },
                    { x: timestamp(1), y: 45 },
                    { x: timestamp(2), y: 40 },
                ],
            },
            {
                name: 'Spike drive',
                data: [
                    { x: timestamp(0), y: 35 },
                    { x: timestamp(2), y: 70 },
                ],
            },
        ]);

        chart = new ApexCharts(host, {
            chart: {
                type: 'area',
                width: 700,
                height: 320,
                animations: { enabled: false },
                toolbar: { show: false },
            },
            series,
            markers: {
                size: 0,
                hover: { sizeOffset: 4 },
            },
            tooltip: {
                shared: true,
                intersect: false,
                custom: createTemperatureChartTooltip(document, 'celsius', '24'),
                fixed: {
                    enabled: true,
                    position: 'topLeft',
                    offsetX: 12,
                    offsetY: 12,
                },
            },
            xaxis: { type: 'datetime' },
        });
        await chart.render();

        await hoverPoint(chart, host, 1, 2);
        const tooltip = host.querySelector<HTMLElement>('.apexcharts-tooltip');
        const initialPosition = { left: tooltip?.style.left, top: tooltip?.style.top };
        const initialHeight = tooltip?.getBoundingClientRect().height;

        await hoverPoint(chart, host, 0, 1);
        expect(visibleTooltipRows(host).length).toBe(series.length);
        expect(visibleTooltipRows(host)[1].textContent).toMatch(/Spike drive:\s*--/);
        expect(tooltip?.getBoundingClientRect().height).toBe(initialHeight);

        await hoverPoint(chart, host, 1, 2);

        const activeRows = Array.from(host.querySelectorAll<HTMLElement>('.apexcharts-tooltip-series-group.apexcharts-active'));
        expect(activeRows.map((row) => row.textContent)).toEqual([jasmine.stringMatching(/Steady drive:\s*40/), jasmine.stringMatching(/Spike drive:\s*70/)]);
        expect(dynamicMarkerPaths(host).every((path) => path.getAttribute('d'))).toBeTrue();
        expect({ left: tooltip?.style.left, top: tooltip?.style.top }).toEqual(initialPosition);
    });

    it('reports an interpolated value for a series sampled less often than its neighbour', async () => {
        // #1076: a padded slot used to read as "--" for every series sampled less
        // often than its neighbours. It now carries an interpolated reading, so this
        // pins what the shared tooltip shows there.
        const timestamp = (hour: number) => new Date(Date.UTC(2026, 7, 1, hour));
        const series = alignTemperatureChartSeries([
            { name: 'Hourly drive', data: [0, 1, 2, 3, 4].map((hour) => ({ x: timestamp(hour), y: 40 })) },
            {
                name: 'Sparse drive',
                data: [
                    { x: timestamp(0), y: 30 },
                    { x: timestamp(2), y: 34 },
                    { x: timestamp(4), y: 38 },
                ],
            },
        ]);

        expect(series[1].data.map((point) => point.y)).toEqual([30, 32, 34, 36, 38]);

        const estimated = estimatedTemperaturePoints(series);
        expect(estimated[0]).toEqual([false, false, false, false, false]);
        expect(estimated[1]).toEqual([false, true, false, true, false]);

        chart = new ApexCharts(host, {
            chart: {
                type: 'area',
                width: 700,
                height: 320,
                animations: { enabled: false },
                toolbar: { show: false },
            },
            series,
            markers: {
                size: 0,
                hover: { sizeOffset: 4 },
            },
            tooltip: {
                shared: true,
                intersect: false,
                custom: createTemperatureChartTooltip(document, 'celsius', '24', (seriesIndex, dataPointIndex) => estimated[seriesIndex]?.[dataPointIndex] === true),
                fixed: {
                    enabled: true,
                    position: 'topLeft',
                    offsetX: 12,
                    offsetY: 12,
                },
            },
            xaxis: { type: 'datetime' },
        });
        await chart.render();

        // Hovering a slot the sparse drive did report: no marker, it is a measurement.
        await hoverPoint(chart, host, 0, 2);
        let rows = visibleTooltipRows(host);
        expect(rows.length).toBe(series.length);
        expect(rows[1].textContent).toMatch(/Sparse drive:\s*34/);
        expect(rows[1].textContent).not.toMatch(/~/);

        // Hovering a padded slot: the value is interpolated and must be marked as such.
        await hoverPoint(chart, host, 0, 1);
        rows = visibleTooltipRows(host);
        expect(rows.length).toBe(series.length);
        expect(rows[1].textContent).toMatch(/Sparse drive:\s*~32/);
        expect(rows[0].textContent).not.toMatch(/~/);
        expect(rows.some((row) => /--/.test(row.textContent ?? ''))).toBeFalse();
    });
});

async function hoverPoint(chart: ApexCharts, host: HTMLElement, seriesIndex: number, dataPointIndex: number): Promise<void> {
    await new Promise<void>((resolve) => setTimeout(resolve, 25));

    const chartContext = chart as any;
    const [x, y] = chartContext.w.globals.pointsArray[seriesIndex][dataPointIndex];
    const gridBounds = host.querySelector<SVGElement>('.apexcharts-grid')?.getBoundingClientRect();
    const svg = host.querySelector<SVGElement>('.apexcharts-svg');

    svg?.dispatchEvent(
        new MouseEvent('mousemove', {
            bubbles: true,
            clientX: (gridBounds?.left || 0) + x,
            clientY: (gridBounds?.top || 0) + y,
        })
    );

    await new Promise<void>((resolve) => setTimeout(resolve, 25));
}

function dynamicMarkerPaths(host: HTMLElement): SVGPathElement[] {
    return Array.from(host.querySelectorAll<SVGPathElement>('.apexcharts-series-markers-wrap > .apexcharts-series-markers:last-child .apexcharts-marker'));
}

function visibleTooltipRows(host: HTMLElement): HTMLElement[] {
    return Array.from(host.querySelectorAll<HTMLElement>('.apexcharts-tooltip-series-group.apexcharts-active')).filter((row) => row.style.display !== 'none');
}
