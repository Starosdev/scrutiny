import { HttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { MDADMService } from './mdadm.service';

describe('MDADMService', () => {
    let service: MDADMService;
    let httpClientSpy: jasmine.SpyObj<HttpClient>;

    beforeEach(() => {
        httpClientSpy = jasmine.createSpyObj('HttpClient', ['get']);
        TestBed.configureTestingModule({
            providers: [MDADMService, { provide: HttpClient, useValue: httpClientSpy }],
        });
        service = TestBed.inject(MDADMService);
    });

    it('normalizes null member devices in summary data', (done: DoneFn) => {
        const response = {
            success: true,
            data: [{ uuid: 'uuid-1', name: 'md0', level: 'raid1', devices: null }],
        } as any;
        httpClientSpy.get.and.returnValue(of(response));

        service.getSummaryData().subscribe((arrays) => {
            expect(arrays[0].devices).toEqual([]);
            done();
        });
    });
});
