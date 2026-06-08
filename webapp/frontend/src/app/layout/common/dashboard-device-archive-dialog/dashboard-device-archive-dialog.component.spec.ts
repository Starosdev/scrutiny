import { ComponentFixture, TestBed } from '@angular/core/testing';

import { DashboardDeviceArchiveDialogComponent } from './dashboard-device-archive-dialog.component';
import { provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { MAT_DIALOG_DATA, MatDialogModule as MatDialogModule, MatDialogRef as MatDialogRef } from '@angular/material/dialog';
import { MatButtonModule as MatButtonModule } from '@angular/material/button';
import { MatIconModule, MatIconRegistry } from '@angular/material/icon';
import { SharedModule } from '../../../shared/shared.module';
import { DashboardDeviceArchiveDialogService } from './dashboard-device-archive-dialog.service';
import { of } from 'rxjs';

describe('DashboardDeviceArchiveDialogComponent', () => {
    let component: DashboardDeviceArchiveDialogComponent;
    let fixture: ComponentFixture<DashboardDeviceArchiveDialogComponent>;

    const matDialogRefSpy = jasmine.createSpyObj('MatDialogRef', ['closeDialog', 'close']);
    const dashboardDeviceArchiveDialogServiceSpy = jasmine.createSpyObj('DashboardDeviceArchiveDialogService', ['archiveDevice']);
    const matIconRegistrySpy = jasmine.createSpyObj('MatIconRegistry', ['getNamedSvgIcon']);

    beforeEach(() => {
        matIconRegistrySpy.getNamedSvgIcon.and.returnValue(of(document.createElementNS('http://www.w3.org/2000/svg', 'svg')));

        TestBed.configureTestingModule({
            imports: [MatDialogModule, MatButtonModule, MatIconModule, SharedModule, DashboardDeviceArchiveDialogComponent],
            providers: [
                { provide: MatDialogRef, useValue: matDialogRefSpy },
                { provide: MAT_DIALOG_DATA, useValue: { deviceId: 'test-device-id', title: 'my-test-device-title' } },
                { provide: DashboardDeviceArchiveDialogService, useValue: dashboardDeviceArchiveDialogServiceSpy },
                { provide: MatIconRegistry, useValue: matIconRegistrySpy },
                provideHttpClient(withInterceptorsFromDi()),
            ],
        }).compileComponents();
    });

    beforeEach(() => {
        fixture = TestBed.createComponent(DashboardDeviceArchiveDialogComponent);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('should close the component if cancel is clicked', () => {
        matDialogRefSpy.closeDialog.calls.reset();
        matDialogRefSpy.closeDialog();
        expect(matDialogRefSpy.closeDialog).toHaveBeenCalled();
    });

    it('should attempt to archive device if archive is clicked', () => {
        dashboardDeviceArchiveDialogServiceSpy.archiveDevice.and.returnValue(of({ success: true }));

        component.onArchiveClick();
        expect(dashboardDeviceArchiveDialogServiceSpy.archiveDevice).toHaveBeenCalledWith('test-device-id');
        expect(dashboardDeviceArchiveDialogServiceSpy.archiveDevice.calls.count()).withContext('one call').toBe(1);
    });
});
