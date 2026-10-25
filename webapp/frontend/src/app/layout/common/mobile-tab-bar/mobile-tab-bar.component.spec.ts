import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NavigationEnd, Router } from '@angular/router';
import { BehaviorSubject, of, Subject } from 'rxjs';
import { DashboardService } from 'app/modules/dashboard/dashboard.service';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { AppConfig, appConfig, MetricsStatusThreshold } from 'app/core/config/app.config';
import { MobileTabBarComponent } from './mobile-tab-bar.component';

describe('MobileTabBarComponent', () => {
    let component: MobileTabBarComponent;
    let fixture: ComponentFixture<MobileTabBarComponent>;
    let routerEvents: Subject<NavigationEnd>;
    let routerSpy: jasmine.SpyObj<Router>;
    let config$: BehaviorSubject<AppConfig>;
    let data$: BehaviorSubject<any>;

    beforeEach(() => {
        routerEvents = new Subject<NavigationEnd>();
        config$ = new BehaviorSubject<AppConfig>(appConfig);
        data$ = new BehaviorSubject<any>(null);
        routerSpy = jasmine.createSpyObj<Router>('Router', ['navigate'], {
            events: routerEvents.asObservable(),
            url: '/mobile-home',
        });

        TestBed.configureTestingModule({
            imports: [MobileTabBarComponent],
            providers: [
                { provide: Router, useValue: routerSpy },
                { provide: DashboardService, useValue: { data$: data$.asObservable(), pageData$: of(null) } },
                { provide: ScrutinyConfigService, useValue: { config$: config$.asObservable() } },
            ],
        }).compileComponents();
    });

    beforeEach(() => {
        fixture = TestBed.createComponent(MobileTabBarComponent);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('should use the dedicated mobile drives route', () => {
        const drivesTab = component.tabs.find((tab) => tab.label === 'Drives');

        expect(drivesTab?.route).toBe('/mobile-drives');
    });

    it('should navigate to the dedicated mobile drives route', () => {
        const drivesTab = component.tabs.find((tab) => tab.label === 'Drives');

        expect(drivesTab).toBeDefined();

        component.navigate(drivesTab!);

        expect(routerSpy.navigate).toHaveBeenCalledWith(['/mobile-drives']);
    });

    describe('drives needing attention', () => {
        const devices = {
            passed: { device: { device_status: 0 } },
            smart: { device: { device_status: 1 } },
            scrutiny: { device: { device_status: 2 } },
            forced: { device: { device_status: 0, has_forced_failure: true } },
            archived: { device: { device_status: 3, archived: true } },
        };

        const countWithThreshold = (threshold: MetricsStatusThreshold): number => {
            config$.next({ ...appConfig, metrics: { ...appConfig.metrics, status_threshold: threshold } });
            data$.next(devices);
            return component.drivesNeedAttention;
        };

        it('counts failures and forced failures with the both threshold', () => {
            expect(countWithThreshold(MetricsStatusThreshold.Both)).toBe(3);
        });

        it('skips failures that the SMART-only threshold hides', () => {
            expect(countWithThreshold(MetricsStatusThreshold.Smart)).toBe(2);
        });

        it('skips failures that the Scrutiny-only threshold hides', () => {
            expect(countWithThreshold(MetricsStatusThreshold.Scrutiny)).toBe(2);
        });

        it('recounts when the threshold changes after the drives load', () => {
            data$.next(devices);
            expect(component.drivesNeedAttention).toBe(3);

            config$.next({ ...appConfig, metrics: { ...appConfig.metrics, status_threshold: MetricsStatusThreshold.Smart } });
            expect(component.drivesNeedAttention).toBe(2);
        });
    });
});
