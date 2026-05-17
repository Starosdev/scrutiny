import { NgModule, inject } from '@angular/core';
import { TreoSplashScreenService } from '@treo/services/splash-screen/splash-screen.service';

@NgModule({
    providers: [TreoSplashScreenService],
})
export class TreoSplashScreenModule {
    private readonly _treoSplashScreenService = inject(TreoSplashScreenService);
}
