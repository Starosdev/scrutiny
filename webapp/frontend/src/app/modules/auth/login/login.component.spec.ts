import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { ActivatedRoute, Router } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { MatIconTestingModule } from '@angular/material/icon/testing';
import { AuthService } from 'app/core/auth/auth.service';
import { LoginComponent } from './login.component';

describe('LoginComponent token guidance', () => {
    let fixture: ComponentFixture<LoginComponent>;

    beforeEach(() => {
        TestBed.configureTestingModule({
            imports: [LoginComponent, MatIconTestingModule],
            providers: [
                provideNoopAnimations(),
                { provide: Router, useValue: { navigate: jasmine.createSpy('navigate'), navigateByUrl: jasmine.createSpy('navigateByUrl') } },
                { provide: ActivatedRoute, useValue: { snapshot: { queryParams: {} } } },
                { provide: AuthService, useValue: { authEnabled: true, isLoggedIn: false, loginMethods$: new BehaviorSubject<string[]>(['token']) } },
            ],
        });
        fixture = TestBed.createComponent(LoginComponent);
        fixture.detectChanges();
    });

    it('explains where the omnibus and hub/spoke token comes from', () => {
        const help: string = fixture.nativeElement.querySelector('.login-help').textContent;
        expect(help).toContain('/opt/scrutiny/config/.auth-token');
        expect(help).toContain('docker exec');
        expect(help).toContain('SCRUTINY_WEB_AUTH_TOKEN');
        expect(fixture.nativeElement.querySelector('.login-help a').getAttribute('href')).toContain('docs/AUTH.md');
    });

    it('announces login errors', () => {
        fixture.componentInstance.errorMessage = 'Invalid token';
        fixture.changeDetectorRef.markForCheck();
        fixture.componentRef.changeDetectorRef.detectChanges();
        expect(fixture.nativeElement.querySelector('.login-error').getAttribute('role')).toBe('alert');
    });
});
