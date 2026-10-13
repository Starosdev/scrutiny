import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatIconTestingModule } from '@angular/material/icon/testing';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, throwError } from 'rxjs';
import { DashboardSettingsComponent } from './dashboard-settings.component';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { AttributeOverrideService } from 'app/core/config/attribute-override.service';
import { DashboardService } from 'app/modules/dashboard/dashboard.service';
import { NotifyUrlService } from 'app/core/config/notify-url.service';
import { AppConfig, appConfig, DEFAULT_TEMPERATURE_THRESHOLD_CELSIUS, DEFAULT_TEMPERATURE_DURATION_MINUTES } from 'app/core/config/app.config';

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

    it('uses valid defaults for invalid hidden values when notifications are disabled', () => {
        component.notifyOnTemperature = false;
        for (const invalid of [null, NaN, Infinity, -1, component.temperatureThresholdMax + 1]) {
            component.temperatureThresholdDisplay = invalid;
            component.saveSettings();
            expect(configService.config?.metrics?.temperature_threshold_celsius).toBe(DEFAULT_TEMPERATURE_THRESHOLD_CELSIUS);
        }
        for (const invalid of [null, NaN, Infinity, -1, 0.5, component.maxTemperatureDurationMinutes + 1]) {
            component.temperatureDurationMinutes = invalid;
            component.saveSettings();
            expect(configService.config?.metrics?.temperature_duration_minutes).toBe(DEFAULT_TEMPERATURE_DURATION_MINUTES);
            expect(configService.config?.metrics?.notify_on_temperature).toBeFalse();
        }
    });

    it('preserves valid hidden settings, including immediate delivery, when disabled', () => {
        component.notifyOnTemperature = false;
        component.setTemperatureUnit('fahrenheit');
        const threshold = 60;
        component.temperatureThresholdCelsius = threshold;
        for (const duration of [0, component.maxTemperatureDurationMinutes]) {
            component.temperatureDurationMinutes = duration;
            component.saveSettings();
            expect(configService.config?.metrics?.temperature_threshold_celsius).toBe(threshold);
            expect(configService.config?.metrics?.temperature_duration_minutes).toBe(duration);
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

describe('DashboardSettingsComponent notification channel fields', () => {
    let component: DashboardSettingsComponent;
    let fixture: ComponentFixture<DashboardSettingsComponent>;

    beforeEach(async () => {
        TestBed.configureTestingModule({
            imports: [DashboardSettingsComponent, MatIconTestingModule],
            providers: [
                { provide: ScrutinyConfigService, useValue: { config$: new BehaviorSubject<AppConfig>(structuredClone(appConfig)) } },
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
        // Width of the settings dialog in the #937 report.
        fixture.nativeElement.style.display = 'block';
        fixture.nativeElement.style.width = '760px';
        document.body.appendChild(fixture.nativeElement);
        component.showAddUrlForm = true;
        component.selectedEngine = 'shoutrrr';
        component.selectedShoutrrrService = 'smtp';
        fixture.detectChanges();
        await fixture.whenStable();
    });

    afterEach(() => fixture.nativeElement.remove());

    it('wraps SMTP fields instead of shrinking them into one row (#937)', () => {
        const host = fixture.nativeElement.querySelector('input[placeholder="smtp.gmail.com"]').closest('mat-form-field');
        const fields: HTMLElement[] = Array.from(host.parentElement.children);
        const rows = new Set(fields.map((field) => Math.round(field.getBoundingClientRect().top)));
        expect(fields.length).toBe(6);
        expect(rows.size).toBeGreaterThan(1);
        for (const field of fields.filter((element) => element.classList.contains('field-lg'))) {
            expect(field.getBoundingClientRect().width).toBeGreaterThanOrEqual(200);
        }
    });
});

describe('DashboardSettingsComponent notification URL errors', () => {
    let component: DashboardSettingsComponent;
    let fixture: ComponentFixture<DashboardSettingsComponent>;
    let notifyService: { addNotifyUrl: jasmine.Spy; testNotifyUrl: jasmine.Spy };

    beforeEach(async () => {
        notifyService = { addNotifyUrl: jasmine.createSpy('addNotifyUrl'), testNotifyUrl: jasmine.createSpy('testNotifyUrl') };
        TestBed.configureTestingModule({
            imports: [DashboardSettingsComponent, MatIconTestingModule],
            providers: [
                { provide: ScrutinyConfigService, useValue: { config$: new BehaviorSubject<AppConfig>(structuredClone(appConfig)) } },
                { provide: AttributeOverrideService, useValue: {} },
                { provide: DashboardService, useValue: {} },
                { provide: NotifyUrlService, useValue: notifyService },
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

    it('shows the server message when saving a rejected URL returns 400', () => {
        notifyService.addNotifyUrl.and.returnValue(throwError(() => ({ status: 400, error: { success: false, error: 'script:// URLs can only be set in the config file.' } })));
        component.selectedEngine = 'shoutrrr';
        component.selectedShoutrrrService = 'custom';
        component.newUrlRaw = 'script:///bin/evil';
        component.showAddUrlForm = true;
        component.addNotifyUrl();
        fixture.detectChanges();
        expect(component.showAddUrlForm).toBeTrue();
        const alert: HTMLElement = fixture.nativeElement.querySelector('[role="alert"]');
        expect(alert.textContent).toContain('script:// URLs can only be set in the config file.');
    });

    it('shows the server message when a test returns 400', () => {
        notifyService.testNotifyUrl.and.returnValue(throwError(() => ({ status: 400, error: { error: 'Unsafe URL' } })));
        component.testNotifyUrl({ id: 3, url: 'x', source: 'ui' } as any);
        expect(component.notifyUrlError).toBe('Unsafe URL');
        expect(component.testingUrlId).toBeNull();
    });

    it('falls back to the errors array, then a generic message', () => {
        expect(component.notifyUrlErrorMessage({ error: { errors: ['boom'] } }, 'x')).toBe('boom');
        expect(component.notifyUrlErrorMessage({}, 'fallback')).toBe('fallback');
    });
});
