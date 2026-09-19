import { Component, OnDestroy, OnInit, ViewEncapsulation, inject, ChangeDetectionStrategy } from '@angular/core';
import { Router, RouterLink, RouterOutlet } from '@angular/router';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { AuthService } from 'app/core/auth/auth.service';
import { ThemeToggleComponent } from '../../common/theme-toggle/theme-toggle.component';
import { MatIconButton } from '@angular/material/button';
import { MatTooltip } from '@angular/material/tooltip';
import { MatIcon } from '@angular/material/icon';
import { MobileTabBarComponent } from '../../common/mobile-tab-bar/mobile-tab-bar.component';
import { CdkScrollable } from '@angular/cdk/scrolling';

@Component({
    selector: 'mobile-layout',
    templateUrl: './mobile-layout.component.html',
    styleUrls: ['./mobile-layout.component.scss'],
    encapsulation: ViewEncapsulation.None,
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [RouterLink, ThemeToggleComponent, MatIconButton, MatTooltip, MatIcon, RouterOutlet, MobileTabBarComponent, CdkScrollable],
})
export class MobileLayoutComponent implements OnInit, OnDestroy {
    private readonly _authService = inject(AuthService);
    private readonly _router = inject(Router);

    authEnabled: boolean = false;

    private readonly _unsubscribeAll: Subject<void>;

    constructor() {
        this._unsubscribeAll = new Subject();
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
