import { HttpClient } from '@angular/common/http';
import { of } from 'rxjs';
import { BtrfsFilesystemDetailService } from './btrfs-filesystem-detail.service';

describe('BtrfsFilesystemDetailService', () => {
    let service: BtrfsFilesystemDetailService;
    let httpClientSpy: jasmine.SpyObj<HttpClient>;

    beforeEach(() => {
        httpClientSpy = jasmine.createSpyObj('HttpClient', ['get', 'post']);
        service = new BtrfsFilesystemDetailService(httpClientSpy);
    });

    it('should return getData()', (done: DoneFn) => {
        const response = { success: true, data: { filesystem: { uuid: 'abc' }, metrics_history: [] } } as any;
        httpClientSpy.get.and.returnValue(of(response));
        service.getData('abc').subscribe(value => {
            expect(value).toEqual(response);
            done();
        });
    });
});
