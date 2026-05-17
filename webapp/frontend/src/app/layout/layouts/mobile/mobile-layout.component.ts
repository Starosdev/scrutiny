import { Component, OnDestroy, OnInit, ViewEncapsulation, inject } from '@angular/core';
import { Router, RouterLink, RouterOutlet } from '@angular/router';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { AuthService } from 'app/core/auth/auth.service';
import { versionInfo } from 'environments/versions';
import { ThemeToggleComponent } from '../../common/theme-toggle/theme-toggle.component';
import { MatIconButton } from '@angular/material/button';
import { MatTooltip } from '@angular/material/tooltip';
import { MatIcon } from '@angular/material/icon';
import { MobileTabBarComponent } from '../../common/mobile-tab-bar/mobile-tab-bar.component';

@Component({
    selector: 'mobile-layout',
    templateUrl: './mobile-layout.component.html',
    styleUrls: ['./mobile-layout.component.scss'],
    encapsulation: ViewEncapsulation.None,
    imports: [RouterLink, ThemeToggleComponent, MatIconButton, MatTooltip, MatIcon, RouterOutlet, MobileTabBarComponent],
})
export class MobileLayoutComponent implements OnInit, OnDestroy {
    private readonly _authService = inject(AuthService);
    private readonly _router = inject(Router);

    appVersion: string;
    authEnabled: boolean = false;

    private _unsubscribeAll: Subject<void>;

    constructor() {
        this._unsubscribeAll = new Subject();
        this.appVersion = versionInfo.version;
    }

    ngOnInit(): void {
        // Redirect to mobile home if landing on dashboard
        if (this._router.url === '/dashboard' || this._router.url === '/') {
            this._router.navigate(['/mobile-home'], { replaceUrl: true });
        }

        this._authService.authEnabled$.pipe(takeUntil(this._unsubscribeAll)).subscribe((enabled) => {
            this.authEnabled = enabled;
        });
    }

    ngOnDestroy(): void {
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    logout(): void {
        this._authService.logout();
    }
}
