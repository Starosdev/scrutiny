import { TestBed } from '@angular/core/testing';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject } from 'rxjs';
import { DashboardSettingsComponent } from './dashboard-settings.component';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { AttributeOverrideService } from 'app/core/config/attribute-override.service';
import { DashboardService } from 'app/modules/dashboard/dashboard.service';
import { NotifyUrlService } from 'app/core/config/notify-url.service';
import { AppConfig, appConfig } from 'app/core/config/app.config';

describe('DashboardSettingsComponent temperature notifications', () => {
    let component: DashboardSettingsComponent;
    let configService: { config$: BehaviorSubject<AppConfig>; config?: AppConfig };

    beforeEach(() => {
        configService = { config$: new BehaviorSubject<AppConfig>(structuredClone(appConfig)) };
        TestBed.configureTestingModule({
            providers: [
                { provide: ScrutinyConfigService, useValue: configService },
                { provide: AttributeOverrideService, useValue: {} },
                { provide: DashboardService, useValue: {} },
                { provide: NotifyUrlService, useValue: {} },
                { provide: HttpClient, useValue: {} },
            ],
        });
        component = TestBed.runInInjectionContext(() => new DashboardSettingsComponent());
        spyOn(component, 'loadOverrides');
        spyOn(component, 'loadOverrideDevices');
        spyOn(component, 'loadNotifyUrls');
        component.ngOnInit();
    });

    it('loads defaults and preserves an immediate duration', () => {
        expect(component.notifyOnTemperature).toBeFalse();
        expect(component.temperatureThresholdCelsius).toBe(55);
        expect(component.temperatureDurationMinutes).toBe(30);
        configService.config$.next({ ...appConfig, metrics: { ...appConfig.metrics, temperature_duration_minutes: 0 } });
        expect(component.temperatureDurationMinutes).toBe(0);
    });

    it('round-trips both units and switches units without changing Celsius', () => {
        component.temperatureThresholdDisplay = 60;
        expect(component.temperatureThresholdCelsius).toBe(60);
        component.temperatureUnit = 'fahrenheit';
        expect(component.temperatureThresholdDisplay).toBe(140);
        component.temperatureThresholdDisplay = 131;
        expect(component.temperatureThresholdCelsius).toBe(55);
        expect(component.temperatureThresholdMin).toBeCloseTo(33.8);
        expect(component.temperatureThresholdMax).toBe(302);
        component.temperatureUnit = 'celsius';
        expect(component.temperatureThresholdDisplay).toBe(55);
        expect(component.temperatureThresholdMin).toBe(1);
        expect(component.temperatureThresholdMax).toBe(150);
    });

    it('saves Celsius and a zero duration', () => {
        component.notifyOnTemperature = true;
        component.temperatureUnit = 'fahrenheit';
        component.temperatureThresholdDisplay = 140;
        component.temperatureDurationMinutes = 0;
        component.saveSettings();
        expect(configService.config?.metrics?.notify_on_temperature).toBeTrue();
        expect(configService.config?.metrics?.temperature_threshold_celsius).toBe(60);
        expect(configService.config?.metrics?.temperature_duration_minutes).toBe(0);
    });

    it('rejects empty and out-of-range thresholds while enabled', () => {
        component.notifyOnTemperature = true;
        for (const invalid of [null, NaN, 0, 151]) {
            component.temperatureThresholdDisplay = invalid;
            expect(component.temperatureSettingsInvalid).toBeTrue();
            component.saveSettings();
            expect(configService.config).toBeUndefined();
        }
        component.notifyOnTemperature = false;
        expect(component.temperatureSettingsInvalid).toBeFalse();
    });

    it('rejects invalid durations', () => {
        component.notifyOnTemperature = true;
        for (const invalid of [null, -1, 0.5, Infinity, 153722868]) {
            component.temperatureDurationMinutes = invalid;
            expect(component.temperatureSettingsInvalid).toBeTrue();
        }
    });
});
