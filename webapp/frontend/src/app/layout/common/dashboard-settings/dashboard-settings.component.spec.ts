import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatIconTestingModule } from '@angular/material/icon/testing';
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
    let fixture: ComponentFixture<DashboardSettingsComponent>;
    let configService: { config$: BehaviorSubject<AppConfig>; config?: AppConfig };

    beforeEach(async () => {
        configService = { config$: new BehaviorSubject<AppConfig>(structuredClone(appConfig)) };
        TestBed.configureTestingModule({
            imports: [DashboardSettingsComponent, MatIconTestingModule],
            providers: [
                { provide: ScrutinyConfigService, useValue: configService },
                { provide: AttributeOverrideService, useValue: {} },
                { provide: DashboardService, useValue: {} },
                { provide: NotifyUrlService, useValue: {} },
                { provide: HttpClient, useValue: {} },
            ],
        });
        fixture = TestBed.createComponent(DashboardSettingsComponent);
        component = fixture.componentInstance;
        spyOn(component, 'loadOverrides');
        spyOn(component, 'loadOverrideDevices');
        spyOn(component, 'loadNotifyUrls');
        fixture.detectChanges();
        await fixture.whenStable();
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
        component.setTemperatureUnit('fahrenheit');
        expect(component.temperatureThresholdDisplay).toBe(140);
        component.temperatureThresholdDisplay = 131;
        expect(component.temperatureThresholdCelsius).toBe(55);
        expect(component.temperatureThresholdMin).toBeCloseTo(33.8);
        expect(component.temperatureThresholdMax).toBe(302);
        component.setTemperatureUnit('celsius');
        expect(component.temperatureThresholdDisplay).toBe(55);
        expect(component.temperatureThresholdMin).toBe(1);
        expect(component.temperatureThresholdMax).toBe(150);
    });

    it('keeps boundary thresholds valid when switching units and leaves invalid values invalid', () => {
        component.notifyOnTemperature = true;
        for (const boundary of [component.temperatureThresholdMin, component.temperatureThresholdMax]) {
            component.temperatureThresholdDisplay = boundary;
            component.setTemperatureUnit('fahrenheit');
            expect(component.temperatureSettingsInvalid).toBeFalse();
            component.setTemperatureUnit('celsius');
            expect(component.temperatureThresholdDisplay).toBe(boundary);
            expect(component.temperatureSettingsInvalid).toBeFalse();
        }
        component.temperatureThresholdDisplay = 0.9;
        component.setTemperatureUnit('fahrenheit');
        component.setTemperatureUnit('celsius');
        expect(component.temperatureSettingsInvalid).toBeTrue();
    });

    it('saves Celsius and a zero duration', () => {
        component.notifyOnTemperature = true;
        component.setTemperatureUnit('fahrenheit');
        component.temperatureThresholdDisplay = 140;
        component.temperatureDurationMinutes = 0;
        component.saveSettings();
        expect(configService.config?.metrics?.notify_on_temperature).toBeTrue();
        expect(configService.config?.metrics?.temperature_threshold_celsius).toBe(60);
        expect(configService.config?.metrics?.temperature_duration_minutes).toBe(0);
    });

    it('rejects empty and out-of-range thresholds while enabled', () => {
        component.notifyOnTemperature = true;
        for (const invalid of [null, NaN, Infinity, 0, 0.9, 150.1, 151]) {
            component.temperatureThresholdDisplay = invalid;
            component.normalizeTemperatureThreshold();
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

    it('preserves fractional and empty values when switching units and rounds on save', () => {
        component.notifyOnTemperature = true;
        component.temperatureThresholdDisplay = 55.5;
        component.setTemperatureUnit('fahrenheit');
        expect(component.temperatureThresholdDisplay).toBeCloseTo(131.9);
        component.setTemperatureUnit('celsius');
        expect(component.temperatureThresholdDisplay).toBeCloseTo(55.5);
        component.saveSettings();
        expect(configService.config?.metrics?.temperature_threshold_celsius).toBe(56);
        component.temperatureThresholdDisplay = null;
        component.setTemperatureUnit('fahrenheit');
        expect(component.temperatureThresholdDisplay).toBeNull();
    });

    it('preserves Fahrenheit keystrokes until blur', async () => {
        configService.config$.next({ ...appConfig, temperature_unit: 'fahrenheit', metrics: { ...appConfig.metrics, notify_on_temperature: true } });
        fixture.changeDetectorRef.markForCheck();
        fixture.detectChanges();
        await fixture.whenStable();
        const input: HTMLInputElement = fixture.nativeElement.querySelector('input[step="any"]');
        for (const value of ['1', '13', '131', '131.5']) {
            input.value = value;
            input.dispatchEvent(new Event('input'));
            fixture.detectChanges();
            await fixture.whenStable();
            expect(input.value).toBe(value);
        }
        input.dispatchEvent(new Event('blur'));
        fixture.detectChanges();
        await fixture.whenStable();
        expect(input.value).toBe('131');
    });

    it('normalizes fractional Celsius only on blur and keeps cleared input invalid', async () => {
        component.notifyOnTemperature = true;
        fixture.changeDetectorRef.markForCheck();
        fixture.detectChanges();
        await fixture.whenStable();
        const input: HTMLInputElement = fixture.nativeElement.querySelector('input[step="any"]');
        input.value = '55.5';
        input.dispatchEvent(new Event('input'));
        fixture.detectChanges();
        await fixture.whenStable();
        expect(input.value).toBe('55.5');
        input.dispatchEvent(new Event('blur'));
        fixture.detectChanges();
        await fixture.whenStable();
        expect(input.value).toBe('56');
        input.value = '';
        input.dispatchEvent(new Event('input'));
        input.dispatchEvent(new Event('blur'));
        fixture.detectChanges();
        await fixture.whenStable();
        expect(input.value).toBe('');
        const saveButton: HTMLButtonElement = fixture.nativeElement.querySelector('button[cdkFocusInitial]');
        expect(saveButton.disabled).toBeTrue();
    });
});
