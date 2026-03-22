import { NgModule, Optional, SkipSelf } from '@angular/core';
import { HTTP_INTERCEPTORS, provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { DomSanitizer } from '@angular/platform-browser';
import { MatIconRegistry } from '@angular/material/icon';
import { AuthInterceptor } from 'app/core/auth/auth.interceptor';

@NgModule({
    imports: [],
    providers: [
        provideHttpClient(withInterceptorsFromDi()),
        { provide: HTTP_INTERCEPTORS, useClass: AuthInterceptor, multi: true }
    ]
})
export class CoreModule
{
    /**
     * Constructor
     *
     * @param {DomSanitizer} _domSanitizer
     * @param {MatIconRegistry} _matIconRegistry
     * @param parentModule
     */
    constructor(
        private readonly _domSanitizer: DomSanitizer,
        private readonly _matIconRegistry: MatIconRegistry,
        @Optional() @SkipSelf() parentModule?: CoreModule
    )
    {
        // Do not allow multiple injections
        if ( parentModule )
        {
            throw new Error('CoreModule has already been loaded. Import this module in the AppModule only.');
        }

        // Register icon sets
        this._matIconRegistry.addSvgIconSet(this._domSanitizer.bypassSecurityTrustResourceUrl('assets/icons/material-twotone.svg'));
        this._matIconRegistry.addSvgIconSetInNamespace('mat_outline', this._domSanitizer.bypassSecurityTrustResourceUrl('assets/icons/material-outline.svg'));
        this._matIconRegistry.addSvgIconSetInNamespace('iconsmind', this._domSanitizer.bypassSecurityTrustResourceUrl('assets/icons/iconsmind.svg'));
        this._matIconRegistry.addSvgIconSetInNamespace('dripicons', this._domSanitizer.bypassSecurityTrustResourceUrl('assets/icons/dripicons.svg'));
        this._matIconRegistry.addSvgIconSetInNamespace('feather', this._domSanitizer.bypassSecurityTrustResourceUrl('assets/icons/feather.svg'));
        this._matIconRegistry.addSvgIconSetInNamespace('heroicons_outline', this._domSanitizer.bypassSecurityTrustResourceUrl('assets/icons/heroicons-outline.svg'));
        this._matIconRegistry.addSvgIconSetInNamespace('heroicons_solid', this._domSanitizer.bypassSecurityTrustResourceUrl('assets/icons/heroicons-solid.svg'));
    }
}
