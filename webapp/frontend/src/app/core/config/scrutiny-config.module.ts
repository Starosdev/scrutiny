import { ModuleWithProviders, NgModule, inject } from '@angular/core';
import { ScrutinyConfigService } from 'app/core/config/scrutiny-config.service';
import { TREO_APP_CONFIG } from '@treo/services/config/config.constants';

@NgModule()
export class ScrutinyConfigModule {
    private readonly _scrutinyConfigService = inject(ScrutinyConfigService);

    /**
     * forRoot method for setting user configuration
     *
     * @param config
     */
    static forRoot(config: any): ModuleWithProviders<ScrutinyConfigModule> {
        return {
            ngModule: ScrutinyConfigModule,
            providers: [
                {
                    provide: TREO_APP_CONFIG,
                    useValue: config,
                },
            ],
        };
    }
}
