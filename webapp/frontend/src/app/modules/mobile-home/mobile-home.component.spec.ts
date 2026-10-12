import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { BehaviorSubject, of } from 'rxjs';
import { DashboardService } from 'app/modules/dashboard/dashboard.service';
import { ZFSPoolsService } from 'app/modules/zfs-pools/zfs-pools.service';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { AppConfig, appConfig, MetricsStatusThreshold } from 'app/core/config/app.config';
import { MobileHomeComponent } from './mobile-home.component';

describe('MobileHomeComponent', () => {
    let component: MobileHomeComponent;
    let fixture: ComponentFixture<MobileHomeComponent>;
    let config$: BehaviorSubject<AppConfig>;
    let data$: BehaviorSubject<any>;

    const devices = {
        passed: { device: { device_status: 0 } },
        smart: { device: { device_status: 1 } },
        scrutiny: { device: { device_status: 2 } },
        both: { device: { device_status: 3 } },
        forced: { device: { device_status: 0, has_forced_failure: true } },
        archived: { device: { device_status: 3, archived: true } },
    };

    const withThreshold = (threshold: MetricsStatusThreshold): void => {
        config$.next({ ...appConfig, metrics: { ...appConfig.metrics, status_threshold: threshold } });
    };

    const attentionStatus = (): Record<string, string> =>
        Object.fromEntries(component.attentionItems.filter((item) => item.type === 'drive').map((item) => [item.id, item.status]));

    beforeEach(() => {
        config$ = new BehaviorSubject<AppConfig>(appConfig);
        data$ = new BehaviorSubject<any>(devices);

        TestBed.configureTestingModule({
            imports: [MobileHomeComponent],
            providers: [
                { provide: Router, useValue: jasmine.createSpyObj<Router>('Router', ['navigate']) },
                { provide: DashboardService, useValue: { data$: data$.asObservable() } },
                { provide: ZFSPoolsService, useValue: { data$: of(null) } },
                { provide: ScrutinyConfigService, useValue: { config$: config$.asObservable() } },
            ],
        }).compileComponents();

        fixture = TestBed.createComponent(MobileHomeComponent);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('classifies drives with the both threshold', () => {
        withThreshold(MetricsStatusThreshold.Both);

        expect(component.driveCounts).toEqual({ healthy: 1, warning: 2, critical: 2 });
        expect(attentionStatus()).toEqual({ smart: 'warning', scrutiny: 'warning', both: 'critical', forced: 'critical' });
    });

    it('treats failures hidden by the SMART-only threshold as healthy', () => {
        withThreshold(MetricsStatusThreshold.Smart);

        expect(component.driveCounts).toEqual({ healthy: 2, warning: 2, critical: 1 });
        expect(attentionStatus()).toEqual({ smart: 'warning', both: 'warning', forced: 'critical' });
    });

    it('treats failures hidden by the Scrutiny-only threshold as healthy', () => {
        withThreshold(MetricsStatusThreshold.Scrutiny);

        expect(component.driveCounts).toEqual({ healthy: 2, warning: 2, critical: 1 });
        expect(attentionStatus()).toEqual({ scrutiny: 'warning', both: 'warning', forced: 'critical' });
    });
});
