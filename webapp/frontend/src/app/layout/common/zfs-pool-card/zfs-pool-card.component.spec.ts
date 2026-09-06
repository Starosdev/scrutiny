import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { MatIconRegistry } from '@angular/material/icon';
import { provideRouter } from '@angular/router';
import { ZFSPoolModel } from 'app/core/models/zfs-pool-model';
import { ZFSPoolsService } from 'app/modules/zfs-pools/zfs-pools.service';
import { of } from 'rxjs';
import { ZFSPoolCardComponent } from './zfs-pool-card.component';

describe('ZFSPoolCardComponent', () => {
    let fixture: ComponentFixture<ZFSPoolCardComponent>;

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
        const iconRegistry = jasmine.createSpyObj<MatIconRegistry>('MatIconRegistry', ['getNamedSvgIcon']);
        iconRegistry.getNamedSvgIcon.and.returnValue(of(document.createElementNS('http://www.w3.org/2000/svg', 'svg')));

        await TestBed.configureTestingModule({
            imports: [ZFSPoolCardComponent],
            providers: [
                provideRouter([]),
                { provide: MatDialog, useValue: {} },
                { provide: MatIconRegistry, useValue: iconRegistry },
                { provide: ZFSPoolsService, useValue: jasmine.createSpyObj('ZFSPoolsService', ['archivePool', 'unarchivePool', 'deletePool']) },
            ],
        }).compileComponents();

        fixture = TestBed.createComponent(ZFSPoolCardComponent);
        fixture.componentRef.setInput('poolSummary', pool);
    });

    it('hides pool mutation menu when server disables modifications', () => {
        fixture.componentRef.setInput('config', { zfs_pool_modifications_allowed: false });
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('[aria-label="Open ZFS pool actions"]')).toBeNull();
    });

    it('shows pool mutation menu when server enables modifications', () => {
        fixture.componentRef.setInput('config', { zfs_pool_modifications_allowed: true });
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelector('[aria-label="Open ZFS pool actions"]')).not.toBeNull();
    });

    it('leads with usable size and shows raw capacity beneath it', () => {
        fixture.componentRef.setInput('poolSummary', { ...pool, usable_used: 25, usable_free: 35 });
        fixture.detectChanges();

        const text = fixture.nativeElement.textContent as string;
        expect(text).toContain('60 B');
        expect(text).toContain('100 B raw');
    });

    it('shows only raw size when the collector reported no usable capacity', () => {
        fixture.detectChanges();

        const text = fixture.nativeElement.textContent as string;
        expect(text).toContain('100 B');
        expect(text).not.toContain('raw');
    });
});
