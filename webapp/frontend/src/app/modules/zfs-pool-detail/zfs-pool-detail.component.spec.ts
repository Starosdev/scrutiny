import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatIconRegistry } from '@angular/material/icon';
import { provideRouter } from '@angular/router';
import { AppConfig } from 'app/core/config/app.config';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { ZFSPoolModel } from 'app/core/models/zfs-pool-model';
import { BehaviorSubject, of } from 'rxjs';
import { ZFSPoolDetailComponent } from './zfs-pool-detail.component';
import { ZFSPoolDetailService } from './zfs-pool-detail.service';

describe('ZFSPoolDetailComponent', () => {
    let fixture: ComponentFixture<ZFSPoolDetailComponent>;
    let config: BehaviorSubject<AppConfig>;

    const pool = {
        guid: '1',
        name: 'tank',
        host_id: 'atlas',
        label: '',
        archived: false,
        muted: false,
        status: 'ONLINE',
        size: 100,
        allocated: 25,
        free: 75,
        fragmentation: 0,
        capacity_percent: 25,
        usable_used: 0,
        usable_free: 0,
        scrub_state: 'none',
        scrub_scanned_bytes: 0,
        scrub_issued_bytes: 0,
        scrub_total_bytes: 0,
        scrub_errors_count: 0,
        scrub_percent_complete: 0,
        total_read_errors: 0,
        total_write_errors: 0,
        total_checksum_errors: 0,
        created_at: '2026-08-01T00:00:00Z',
        updated_at: '2026-08-01T00:00:00Z',
    } satisfies ZFSPoolModel;

    beforeEach(async () => {
        config = new BehaviorSubject<AppConfig>({ zfs_pool_modifications_allowed: false });
        const iconRegistry = jasmine.createSpyObj<MatIconRegistry>('MatIconRegistry', ['getNamedSvgIcon']);
        iconRegistry.getNamedSvgIcon.and.returnValue(of(document.createElementNS('http://www.w3.org/2000/svg', 'svg')));

        await TestBed.configureTestingModule({
            imports: [ZFSPoolDetailComponent],
            providers: [
                provideRouter([]),
                { provide: MatIconRegistry, useValue: iconRegistry },
                { provide: ScrutinyConfigService, useValue: { config$: config.asObservable() } },
                {
                    provide: ZFSPoolDetailService,
                    useValue: {
                        data$: of({ success: true, data: { pool, metrics_history: [] } }),
                        setMuted: jasmine.createSpy('setMuted').and.returnValue(of({ success: true })),
                    },
                },
            ],
        }).compileComponents();

        fixture = TestBed.createComponent(ZFSPoolDetailComponent);
        fixture.detectChanges();
    });

    it('hides mute control when server disables modifications', () => {
        expect(fixture.nativeElement.querySelector('[data-testid="zfs-pool-mute-toggle"]')).toBeNull();
    });

    it('shows mute control when server enables modifications', () => {
        config.next({ zfs_pool_modifications_allowed: true });
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('[data-testid="zfs-pool-mute-toggle"]')).not.toBeNull();
    });

    it('shows missing inventory instead of historical online status', () => {
        fixture.componentInstance.pool = { ...pool, presence: 'missing' };
        fixture.detectChanges();

        const text = fixture.nativeElement.textContent as string;
        expect(text).toContain('MISSING');
        expect(text).toContain('Missing from last inventory');
    });
});
