import { Component, OnInit, inject } from '@angular/core';
import { MAT_DIALOG_DATA, MatDialogRef, MatDialogTitle, MatDialogContent, MatDialogActions } from '@angular/material/dialog';
import { ApexOptions, ChartComponent } from 'ng-apexcharts';
import { CdkScrollable } from '@angular/cdk/scrolling';
import { NgClass } from '@angular/common';
import { MatButton } from '@angular/material/button';

export interface AttributeHistoryData {
    attributeName: string;
    chartData: Array<{
        name: string;
        data: Array<{ x: string; y: number; strokeColor?: string; fillColor?: string }>;
    }>;
    isDark: boolean;
}

@Component({
    selector: 'attribute-history-dialog',
    templateUrl: './attribute-history-dialog.component.html',
    styleUrls: ['./attribute-history-dialog.component.scss'],
    imports: [MatDialogTitle, CdkScrollable, MatDialogContent, ChartComponent, NgClass, MatDialogActions, MatButton],
})
export class AttributeHistoryDialogComponent implements OnInit {
    dialogRef = inject<MatDialogRef<AttributeHistoryDialogComponent>>(MatDialogRef);
    data = inject<AttributeHistoryData>(MAT_DIALOG_DATA);

    chartOptions: Partial<ApexOptions>;
    historyData: Array<{ date: string; value: number; status: string }> = [];

    ngOnInit(): void {
        this._prepareChartOptions();
        this._prepareHistoryData();
    }

    private _prepareChartOptions(): void {
        this.chartOptions = {
            chart: {
                type: 'bar',
                height: 200,
                toolbar: {
                    show: false,
                },
                animations: {
                    enabled: false,
                },
            },
            plotOptions: {
                bar: {
                    columnWidth: '80%',
                },
            },
            series: this.data.chartData,
            xaxis: {
                type: 'category',
                labels: {
                    show: false,
                },
            },
            yaxis: {
                labels: {
                    style: {
                        colors: this.data.isDark ? '#9ca3af' : '#6b7280',
                    },
                },
            },
            tooltip: {
                enabled: true,
                theme: this.data.isDark ? 'dark' : 'light',
                x: {
                    show: true,
                },
                y: {
                    title: {
                        formatter: () => '',
                    },
                },
            },
            stroke: {
                width: 2,
                colors: ['#667EEA'],
            },
            grid: {
                borderColor: this.data.isDark ? '#374151' : '#e5e7eb',
            },
        };
    }

    private _prepareHistoryData(): void {
        if (this.data.chartData && this.data.chartData[0]?.data) {
            this.historyData = this.data.chartData[0].data.map((point) => {
                let status = 'passed';
                if (point.fillColor === '#F05252') {
                    status = 'failed';
                } else if (point.fillColor === '#C27803') {
                    status = 'warn';
                }
                return {
                    date: point.x,
                    value: point.y,
                    status,
                };
            });
        }
    }

    close(): void {
        this.dialogRef.close();
    }
}
