import { HttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { BtrfsFilesystemDetailService } from './btrfs-filesystem-detail.service';

describe('BtrfsFilesystemDetailService', () => {
    let service: BtrfsFilesystemDetailService;
    let httpClientSpy: jasmine.SpyObj<HttpClient>;

    beforeEach(() => {
        httpClientSpy = jasmine.createSpyObj('HttpClient', ['get', 'post']);
        TestBed.configureTestingModule({
            providers: [BtrfsFilesystemDetailService, { provide: HttpClient, useValue: httpClientSpy }],
        });
        service = TestBed.inject(BtrfsFilesystemDetailService);
    });

    it('should return getData()', (done: DoneFn) => {
        const response = { success: true, data: { filesystem: { uuid: 'abc' }, metrics_history: [] } } as any;
        httpClientSpy.get.and.returnValue(of(response));
        service.getData('abc').subscribe((value) => {
            expect(value).toEqual(response);
            done();
        });
    });
});
