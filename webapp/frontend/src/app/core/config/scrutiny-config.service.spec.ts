import { HttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { TREO_APP_CONFIG } from '@treo/services/config/config.constants';
import { of, Subject } from 'rxjs';
import { AppConfig, appConfig } from './app.config';
import { ScrutinyConfigService } from './scrutiny-config.service';

describe('ScrutinyConfigService', () => {
    let service: ScrutinyConfigService;
    let httpClientSpy: jasmine.SpyObj<HttpClient>;

    beforeEach(() => {
        httpClientSpy = jasmine.createSpyObj('HttpClient', ['get', 'post']);
        TestBed.configureTestingModule({
            providers: [ScrutinyConfigService, { provide: HttpClient, useValue: httpClientSpy }, { provide: TREO_APP_CONFIG, useValue: appConfig }],
        });
        service = TestBed.inject(ScrutinyConfigService);
    });

    it('keeps ZFS pool mutation controls hidden until server enables them', () => {
        const remoteConfig = new Subject<any>();
        let latestConfig: AppConfig;
        httpClientSpy.get.and.returnValue(remoteConfig);

        service.config$.subscribe((config) => (latestConfig = config));
        expect(latestConfig.zfs_pool_modifications_allowed).toBeFalse();

        remoteConfig.next({ settings: {}, zfs_pool_modifications_allowed: true });
        expect(latestConfig.zfs_pool_modifications_allowed).toBeTrue();
    });

    it('preserves server ZFS capability after saving dashboard settings', () => {
        httpClientSpy.get.and.returnValue(of({ settings: {}, zfs_pool_modifications_allowed: true }));
        httpClientSpy.post.and.returnValue(of({ settings: { theme: 'dark' }, zfs_pool_modifications_allowed: true }));
        let latestConfig: AppConfig;

        service.config$.subscribe((config) => (latestConfig = config));
        service.config = { theme: 'dark' };

        expect(latestConfig.theme).toBe('dark');
        expect(latestConfig.zfs_pool_modifications_allowed).toBeTrue();
    });

    it('loads the backend server version for runtime display', () => {
        httpClientSpy.get.and.returnValue(of({ settings: {}, server_version: '1.73.0' }));
        let latestConfig: AppConfig;

        service.config$.subscribe((config) => (latestConfig = config));

        expect(latestConfig.server_version).toBe('1.73.0');
    });
});
