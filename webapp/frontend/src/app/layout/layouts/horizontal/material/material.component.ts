import { Component, HostBinding, OnDestroy, OnInit, ViewEncapsulation, inject } from '@angular/core';
import { ActivatedRoute, Data, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { TreoMediaWatcherService } from '@treo/services/media-watcher';
import { TreoNavigationService } from '@treo/components/navigation';
import { AuthService } from 'app/core/auth/auth.service';
import { versionInfo } from 'environments/versions';
import { AppConfig } from 'app/core/config/app.config';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { TreoVerticalNavigationComponent } from '../../../../../@treo/components/navigation/vertical/vertical.component';
import { ThemeToggleComponent } from '../../../common/theme-toggle/theme-toggle.component';
import { MatIconButton } from '@angular/material/button';
import { MatTooltip } from '@angular/material/tooltip';
import { MatIcon } from '@angular/material/icon';
import { MaterialLayoutModule } from './material.module';

@Component({
    selector: 'material-layout',
    templateUrl: './material.component.html',
    styleUrls: ['./material.component.scss'],
    encapsulation: ViewEncapsulation.None,
    imports: [TreoVerticalNavigationComponent, RouterLink, RouterLinkActive, ThemeToggleComponent, MatIconButton, MatTooltip, MatIcon, RouterOutlet, MaterialLayoutModule],
})
export class MaterialLayoutComponent implements OnInit, OnDestroy {
    private readonly _activatedRoute = inject(ActivatedRoute);
    private readonly _authService = inject(AuthService);
    private readonly _configService = inject(ScrutinyConfigService);
    private readonly _treoMediaWatcherService = inject(TreoMediaWatcherService);
    private readonly _treoNavigationService = inject(TreoNavigationService);
    private readonly _router = inject(Router);

    appVersion: string;
    authEnabled: boolean = false;
    config: AppConfig = {};
    data: any;
    isScreenSmall: boolean;

    @HostBinding('class.fixed-header')
    fixedHeader: boolean;

    @HostBinding('class.fixed-footer')
    fixedFooter: boolean;

    // Private
    private readonly _unsubscribeAll: Subject<void>;

    /**
     * Constructor
     *
     * @param {ActivatedRoute} _activatedRoute
     * @param {TreoMediaWatcherService} _treoMediaWatcherService
     * @param {TreoNavigationService} _treoNavigationService
     * @param {Router} _router
     */
    constructor() {
        // Set the private defaults
        this._unsubscribeAll = new Subject();

        // Set the defaults
        this.fixedHeader = false;
        this.fixedFooter = false;

        this.appVersion = versionInfo.version;
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Accessors
    // -----------------------------------------------------------------------------------------------------

    /**
     * Getter for current year
     */
    get currentYear(): number {
        return new Date().getFullYear();
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Lifecycle hooks
    // -----------------------------------------------------------------------------------------------------

    /**
     * On init
     */
    ngOnInit(): void {
        // Subscribe to the resolved route data
        this._activatedRoute.data.subscribe((data: Data) => {
            this.data = data.initialData;
        });

        // Subscribe to auth state
        this._authService.authEnabled$.pipe(takeUntil(this._unsubscribeAll)).subscribe((enabled) => {
            this.authEnabled = enabled;
        });

        // Subscribe to config changes
        this._configService.config$.pipe(takeUntil(this._unsubscribeAll)).subscribe((config: AppConfig) => {
            this.config = config;
        });

        // Subscribe to media changes
        this._treoMediaWatcherService.onMediaChange$.pipe(takeUntil(this._unsubscribeAll)).subscribe(({ matchingAliases }) => {
            // Check if the breakpoint is 'lt-md'
            this.isScreenSmall = matchingAliases.includes('lt-md');
        });
    }

    /**
     * On destroy
     */
    ngOnDestroy(): void {
        // Unsubscribe from all subscriptions
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    // -----------------------------------------------------------------------------------------------------
    // @ Public methods
    // -----------------------------------------------------------------------------------------------------

    /**
     * Toggle navigation
     *
     * @param key
     */
    toggleNavigation(key): void {
        // Get the navigation
        const navigation = this._treoNavigationService.getComponent(key);

        if (navigation) {
            // Toggle the opened status
            navigation.toggle();
        }
    }

    logout(): void {
        this._authService.logout();
    }
}
