import { HttpClient } from '@angular/common/http';
import { of } from 'rxjs';
import { BtrfsFilesystemsService } from './btrfs-filesystems.service';

describe('BtrfsFilesystemsService', () => {
    let service: BtrfsFilesystemsService;
    let httpClientSpy: jasmine.SpyObj<HttpClient>;

    beforeEach(() => {
        httpClientSpy = jasmine.createSpyObj('HttpClient', ['get', 'post', 'delete']);
        service = new BtrfsFilesystemsService(httpClientSpy);
    });

    it('should unwrap and return getSummaryData()', (done: DoneFn) => {
        const response = { success: true, data: { filesystems: { abc: { uuid: 'abc' } } } } as any;
        httpClientSpy.get.and.returnValue(of(response));
        service.getSummaryData().subscribe(value => {
            expect(value).toEqual(response.data.filesystems);
            done();
        });
    });
});
