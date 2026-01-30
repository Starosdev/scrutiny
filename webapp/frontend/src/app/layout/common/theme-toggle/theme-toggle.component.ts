import { Component, OnDestroy, OnInit, ViewEncapsulation } from '@angular/core';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { AppConfig, Theme } from 'app/core/config/app.config';

@Component({
    selector: 'theme-toggle',
    templateUrl: './theme-toggle.component.html',
    styleUrls: ['./theme-toggle.component.scss'],
    encapsulation: ViewEncapsulation.None,
    standalone: false
})
export class ThemeToggleComponent implements OnInit, OnDestroy {
    currentTheme: Theme = 'light';

    private _unsubscribeAll: Subject<void> = new Subject();

    constructor(private _configService: ScrutinyConfigService) {}

    ngOnInit(): void {
        this._configService.config$
            .pipe(takeUntil(this._unsubscribeAll))
            .subscribe((config: AppConfig) => {
                this.currentTheme = config.theme || 'light';
            });
    }

    ngOnDestroy(): void {
        this._unsubscribeAll.next();
        this._unsubscribeAll.complete();
    }

    get currentIcon(): string {
        switch (this.currentTheme) {
            case 'light': return 'heroicons_outline:sun';
            case 'dark': return 'heroicons_outline:moon';
            case 'system': return 'heroicons_outline:desktop-computer';
            default: return 'heroicons_outline:sun';
        }
    }

    get currentLabel(): string {
        switch (this.currentTheme) {
            case 'light': return 'Light';
            case 'dark': return 'Dark';
            case 'system': return 'System';
            default: return 'Light';
        }
    }

    cycleTheme(): void {
        const order: Theme[] = ['light', 'dark', 'system'];
        const currentIndex = order.indexOf(this.currentTheme);
        const nextIndex = (currentIndex + 1) % order.length;
        this._configService.config = { theme: order[nextIndex] };
    }
}
