import { Component, OnDestroy, OnInit, ViewEncapsulation } from '@angular/core';
import { Router } from '@angular/router';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { AuthService } from 'app/core/auth/auth.service';
import { versionInfo } from 'environments/versions';

@Component({
    selector: 'mobile-layout',
    templateUrl: './mobile-layout.component.html',
    styleUrls: ['./mobile-layout.component.scss'],
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class MobileLayoutComponent implements OnInit, OnDestroy {
    appVersion: string;
    authEnabled: boolean = false;

    private _unsubscribeAll: Subject<void>;

    constructor(private _authService: AuthService, private _router: Router) {
        this._unsubscribeAll = new Subject();
        this.appVersion = versionInfo.version;
    }

    ngOnInit(): void {
        // Redirect to mobile home if landing on dashboard
        if (this._router.url === '/dashboard' || this._router.url === '/') {
            this._router.navigate(['/mobile-home'], { replaceUrl: true });
        }

        this._authService.authEnabled$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(enabled => {
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
