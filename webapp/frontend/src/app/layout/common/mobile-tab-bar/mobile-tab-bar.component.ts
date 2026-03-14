import { Component, OnDestroy, OnInit, ViewEncapsulation } from '@angular/core';
import { NavigationEnd, Router } from '@angular/router';
import { Subject } from 'rxjs';
import { filter, takeUntil } from 'rxjs/operators';
import { DashboardService } from 'app/modules/dashboard/dashboard.service';

interface TabItem {
    icon: string;
    label: string;
    route: string;
    exactMatch?: boolean;
}

@Component({
    selector: 'mobile-tab-bar',
    templateUrl: './mobile-tab-bar.component.html',
    styleUrls: ['./mobile-tab-bar.component.scss'],
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class MobileTabBarComponent implements OnInit, OnDestroy {
    tabs: TabItem[] = [
        { icon: 'home', label: 'Home', route: '/mobile-home', exactMatch: true },
        { icon: 'storage', label: 'Drives', route: '/dashboard', exactMatch: true },
        { icon: 'dns', label: 'ZFS', route: '/zfs-pools', exactMatch: true },
        { icon: 'speed', label: 'Workload', route: '/workload', exactMatch: true },
        { icon: 'settings', label: 'Settings', route: '/mobile-settings', exactMatch: true },
    ];

    activeRoute: string = '';
    isDetailPage: boolean = false;
    drivesNeedAttention: number = 0;

    private _unsubscribeAll: Subject<void>;

    constructor(private _router: Router, private readonly _dashboardService: DashboardService) {
        this._unsubscribeAll = new Subject();
    }

    ngOnInit(): void {
        this.activeRoute = this._router.url;
        this.isDetailPage = this._isDetailRoute(this._router.url);

        this._router.events.pipe(
            filter(event => event instanceof NavigationEnd),
            takeUntil(this._unsubscribeAll)
        ).subscribe((event: NavigationEnd) => {
            this.activeRoute = event.urlAfterRedirects;
            this.isDetailPage = this._isDetailRoute(event.urlAfterRedirects);
        });

        this._dashboardService.data$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe(data => {
                if (data) {
                    this.drivesNeedAttention = 0;
                    for (const wwn in data) {
                        const status = data[wwn].device?.device_status;
                        if (status && status > 0 && !data[wwn].device?.archived) {
                            this.drivesNeedAttention++;
                        }
                    }
                }
            });
    }

    ngOnDestroy(): void {
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    isActive(tab: TabItem): boolean {
        return this.activeRoute.startsWith(tab.route);
    }

    navigate(tab: TabItem): void {
        this._router.navigate([tab.route]);
    }

    private _isDetailRoute(url: string): boolean {
        return url.startsWith('/device/') || url.startsWith('/zfs-pool/');
    }
}
