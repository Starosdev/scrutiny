import {
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    OnDestroy,
    OnInit,
    ViewEncapsulation
} from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { ApexOptions } from 'ng-apexcharts';
import { MDADMService } from 'app/modules/mdadm/mdadm.service';
import { MDADMArrayModel, MDADMMetricsHistoryModel } from 'app/core/models/mdadm-array-model';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { AppConfig } from 'app/core/config/app.config';
import { apexShortDateTime } from 'app/shared/time-format.utils';

@Component({
    selector: 'mdadm-detail',
    templateUrl: './mdadm-detail.component.html',
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class MDADMDetailComponent implements OnInit, OnDestroy {
    array: MDADMArrayModel;
    history: MDADMMetricsHistoryModel[];
    chartOptions: ApexOptions;
    config: AppConfig;

    private _unsubscribeAll: Subject<void>;

    constructor(
        private readonly _mdadmService: MDADMService,
        private readonly _configService: ScrutinyConfigService,
        private readonly _route: ActivatedRoute,
        private readonly _router: Router,
        private readonly _changeDetectorRef: ChangeDetectorRef
    ) {
        this._unsubscribeAll = new Subject();
    }

    ngOnInit(): void {
        const uuid = this._route.snapshot.paramMap.get('uuid');

        this._configService.config$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((config) => {
                this.config = config;
                this._changeDetectorRef.markForCheck();
            });

        this._mdadmService.getDetails(uuid)
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((response) => {
                this.array = response.data.array;
                this.history = response.data.history;
                this._prepareChartData();
                this._changeDetectorRef.markForCheck();
            });
    }

    ngOnDestroy(): void {
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    private _prepareChartData(): void {
        if (!this.history || this.history.length === 0) {
            return;
        }

        const activeData = this.history.map(m => ({ x: new Date(m.date), y: m.active_devices }));
        const failedData = this.history.map(m => ({ x: new Date(m.date), y: m.failed_devices }));

        this.chartOptions = {
            chart: {
                animations: {
                    speed: 400,
                    animateGradually: { enabled: false }
                },
                fontFamily: 'inherit',
                foreColor: 'inherit',
                width: '100%',
                height: 350,
                type: 'line',
                sparkline: { enabled: false }
            },
            colors: ['#38a169', '#e53e3e'], // Green for active, Red for failed
            series: [
                { name: 'Active Devices', data: activeData },
                { name: 'Failed Devices', data: failedData }
            ],
            stroke: {
                curve: 'smooth',
                width: 3
            },
            tooltip: {
                theme: 'dark',
                x: {
                    format: apexShortDateTime(this.config.time_format, true)
                }
            },
            xaxis: {
                type: 'datetime',
                labels: { datetimeUTC: false }
            },
            yaxis: {
                min: 0,
                forceNiceScale: true
            },
            legend: {
                position: 'top',
                horizontalAlign: 'right'
            }
        };
    }

    goBack(): void {
        this._router.navigate(['/mdadm']);
    }
}
