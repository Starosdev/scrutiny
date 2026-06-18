import { HttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { DeviceSelfTestsResponseWrapper } from 'app/core/models/device-selftests-response-wrapper';
import { DetailService } from './detail.service';
import { of } from 'rxjs';
import { sda } from 'app/data/mock/device/details/sda';
import { DeviceDetailsResponseWrapper } from 'app/core/models/device-details-response-wrapper';

describe('DetailService', () => {
    describe('#getData', () => {
        let service: DetailService;
        let httpClientSpy: jasmine.SpyObj<HttpClient>;

        beforeEach(() => {
            httpClientSpy = jasmine.createSpyObj('HttpClient', ['get']);
            TestBed.configureTestingModule({
                providers: [DetailService, { provide: HttpClient, useValue: httpClientSpy }],
            });
            service = TestBed.inject(DetailService);
        });
        it('should return getData() (HttpClient called once)', (done: DoneFn) => {
            httpClientSpy.get.and.returnValue(of(sda));

            service.getData('test').subscribe((value) => {
                expect(value).toBe(sda as DeviceDetailsResponseWrapper);
                done();
            });
            expect(httpClientSpy.get.calls.count()).withContext('one call').toBe(1);
        });
    });

    describe('#getSelfTestData', () => {
        let service: DetailService;
        let httpClientSpy: jasmine.SpyObj<HttpClient>;

        beforeEach(() => {
            httpClientSpy = jasmine.createSpyObj('HttpClient', ['get']);
            TestBed.configureTestingModule({
                providers: [DetailService, { provide: HttpClient, useValue: httpClientSpy }],
            });
            service = TestBed.inject(DetailService);
        });

        it('should return getSelfTestData() (HttpClient called once)', (done: DoneFn) => {
            const response: DeviceSelfTestsResponseWrapper = {
                success: true,
                data: {
                    self_tests: [],
                },
            };
            httpClientSpy.get.and.returnValue(of(response));

            service.getSelfTestData('test').subscribe((value) => {
                expect(value).toBe(response);
                done();
            });
            expect(httpClientSpy.get.calls.count()).withContext('one call').toBe(1);
        });
    });
});
