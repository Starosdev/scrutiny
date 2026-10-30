import { HttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { MDADMService } from './mdadm.service';

describe('MDADMService', () => {
    let service: MDADMService;
    let httpClientSpy: jasmine.SpyObj<HttpClient>;

    beforeEach(() => {
        httpClientSpy = jasmine.createSpyObj('HttpClient', ['get', 'post', 'delete']);
        TestBed.configureTestingModule({
            providers: [MDADMService, { provide: HttpClient, useValue: httpClientSpy }],
        });
        service = TestBed.inject(MDADMService);
    });

    it('posts to the archive route, with the identifier escaped', (done: DoneFn) => {
        httpClientSpy.post.and.returnValue(of({ success: true }));

        service.archiveArray('synthetic:4b825dc6').subscribe(() => {
            expect(httpClientSpy.post.calls.mostRecent().args[0]).toContain('/api/mdadm/array/synthetic%3A4b825dc6/archive');
            done();
        });
    });

    it('posts to the unmute route', (done: DoneFn) => {
        httpClientSpy.post.and.returnValue(of({ success: true }));

        service.unmuteArray('uuid-1').subscribe(() => {
            expect(httpClientSpy.post.calls.mostRecent().args[0]).toContain('/api/mdadm/array/uuid-1/unmute');
            done();
        });
    });

    it('sends the label in the body', (done: DoneFn) => {
        httpClientSpy.post.and.returnValue(of({ success: true }));

        service.setLabel('uuid-1', 'Media array').subscribe(() => {
            const [url, body] = httpClientSpy.post.calls.mostRecent().args;
            expect(url).toContain('/api/mdadm/array/uuid-1/label');
            expect(body).toEqual({ label: 'Media array' });
            done();
        });
    });

    it('deletes an array by identifier', (done: DoneFn) => {
        httpClientSpy.delete.and.returnValue(of({ success: true }));

        service.deleteArray('uuid-1').subscribe(() => {
            expect(httpClientSpy.delete.calls.mostRecent().args[0]).toContain('/api/mdadm/array/uuid-1');
            done();
        });
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
