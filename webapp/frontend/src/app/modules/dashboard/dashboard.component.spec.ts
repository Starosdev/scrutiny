import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { Router } from '@angular/router';
import { of } from 'rxjs';
import { AppConfig } from 'app/core/config/app.config';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { DashboardComponent } from './dashboard.component';
import { DashboardService } from './dashboard.service';
import { TemperatureSelection } from './temperature-selection';

describe('DashboardComponent temperature chart', () => {
    let component: DashboardComponent;
    let fixture: ComponentFixture<DashboardComponent>;
    let dashboardService: jasmine.SpyObj<DashboardService>;

    beforeEach(() => {
        const config: AppConfig = {
            dashboard_columns: 2,
            dashboard_density: 'comfortable',
            line_stroke: 'smooth',
            temperature_unit: 'celsius',
            theme: 'light',
            time_format: '24',
        };
        const temperatureSelection = new TemperatureSelection({
            getItem: () => null,
            setItem: () => undefined,
        });
        spyOn(temperatureSelection, 'restore').and.callThrough();

        dashboardService = jasmine.createSpyObj('DashboardService', ['getSummaryPage', 'getTemperatureDeviceOptions', 'getSummaryTempData', 'getFilesystemSummaryData'], {
            pageData$: of({
                summary: {},
                pagination: {
                    page: 1,
                    page_size: 10,
                    total_items: 0,
                    total_pages: 0,
                    attention_count: 0,
                },
            }),
            temperatureSelection,
        });
        dashboardService.getTemperatureDeviceOptions.and.returnValue(
            of(
                Array.from({ length: 19 }, (_, index) => ({
                    device_id: `drive-${index + 1}`,
                    host_id: 'host-1',
                    label: '',
                    device_name: `sd${String.fromCharCode(97 + index)}`,
                    model_name: 'Test Drive',
                    serial_number: `serial-${index + 1}`,
                }))
            )
        );
        dashboardService.getSummaryTempData.and.returnValue(
            of({
                'drive-1': [{ date: '2026-08-03T12:00:00Z', temp: 64 }],
            })
        );
        dashboardService.getFilesystemSummaryData.and.returnValue(of({ filesystems: {}, hosts: {} }));
        dashboardService.getSummaryPage.and.returnValue(
            of({
                summary: {},
                pagination: {
                    page: 1,
                    page_size: 10,
                    total_items: 0,
                    total_pages: 0,
                    attention_count: 0,
                },
            })
        );

        TestBed.configureTestingModule({
            imports: [DashboardComponent],
            providers: [
                { provide: DashboardService, useValue: dashboardService },
                { provide: ScrutinyConfigService, useValue: { config$: of(config) } },
                { provide: MatDialog, useValue: { open: jasmine.createSpy('open') } },
                {
                    provide: Router,
                    useValue: {
                        navigate: jasmine.createSpy('navigate'),
                        onSameUrlNavigation: 'ignore',
                        routeReuseStrategy: { shouldReuseRoute: () => true },
                        url: '/web/dashboard',
                    },
                },
            ],
        });

        fixture = TestBed.createComponent(DashboardComponent);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    afterEach(() => {
        fixture.destroy();
    });

    it('shows selected drives out of available drives', () => {
        const driveFilterButton = Array.from<HTMLElement>(fixture.nativeElement.querySelectorAll('button')).find((button) => button.textContent?.includes('Drives ('));

        expect(driveFilterButton?.textContent).toContain('Drives (0/19)');
        expect(driveFilterButton?.textContent).not.toContain(`/${component.temperatureSelection.maxSelected})`);
    });

    it('restores selection against active temperature devices', () => {
        expect(component.temperatureSelection.restore).toHaveBeenCalledWith(jasmine.arrayContaining(['drive-1']));
    });

    it('binds loaded temperature data before the lazy chart instance exists', () => {
        component.toggleTemperatureDevice('drive-1');

        expect(component.temperatureOptions.series).toEqual([
            {
                name: 'host-1: /dev/sda - Test Drive',
                data: [{ x: new Date('2026-08-03T12:00:00Z'), y: 64 }],
            },
        ]);
    });

    it('keeps current page after archiving when that page still exists', () => {
        component.pagination = {
            page: 2,
            page_size: 25,
            total_items: 75,
            total_pages: 3,
            attention_count: 0,
        };

        component.onDeviceArchived('drive-1');

        expect(dashboardService.getSummaryPage).toHaveBeenCalledWith(jasmine.objectContaining({ page: 2, groupBy: 'host', pageSize: 10 }));
    });

    it('loads the new last page after archiving empties the current page', () => {
        const page = (currentPage: number, totalPages: number) =>
            of({
                summary: {},
                pagination: {
                    page: currentPage,
                    page_size: 10,
                    total_items: 20,
                    total_pages: totalPages,
                    attention_count: 0,
                },
            });
        dashboardService.getSummaryPage.and.returnValues(page(3, 2), page(2, 2));
        component.pagination = {
            page: 3,
            page_size: 10,
            total_items: 21,
            total_pages: 3,
            attention_count: 0,
        };

        component.onDeviceArchived('drive-1');

        expect(dashboardService.getSummaryPage.calls.argsFor(0)[0]).toEqual(jasmine.objectContaining({ page: 3 }));
        expect(dashboardService.getSummaryPage.calls.argsFor(1)[0]).toEqual(jasmine.objectContaining({ page: 2 }));
    });

    it('applies trimmed host search across paginated dashboard results', () => {
        component.hostSearch = ' host-42 ';

        component.applyHostSearch();

        expect(dashboardService.getSummaryPage).toHaveBeenCalledWith(jasmine.objectContaining({ page: 1, hostSearch: 'host-42' }));
    });
});
