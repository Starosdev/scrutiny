import { ComponentFixture, TestBed } from '@angular/core/testing';
import { NavigationEnd, Router } from '@angular/router';
import { of, Subject } from 'rxjs';
import { DashboardService } from 'app/modules/dashboard/dashboard.service';
import { MobileTabBarComponent } from './mobile-tab-bar.component';

describe('MobileTabBarComponent', () => {
    let component: MobileTabBarComponent;
    let fixture: ComponentFixture<MobileTabBarComponent>;
    let routerEvents: Subject<NavigationEnd>;
    let routerSpy: jasmine.SpyObj<Router>;

    beforeEach(() => {
        routerEvents = new Subject<NavigationEnd>();
        routerSpy = jasmine.createSpyObj<Router>('Router', ['navigate'], {
            events: routerEvents.asObservable(),
            url: '/mobile-home',
        });

        TestBed.configureTestingModule({
            imports: [MobileTabBarComponent],
            providers: [
                { provide: Router, useValue: routerSpy },
                { provide: DashboardService, useValue: { data$: of(null) } },
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
});
