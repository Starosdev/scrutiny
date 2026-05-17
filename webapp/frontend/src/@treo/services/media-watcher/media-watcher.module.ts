import { NgModule, inject } from '@angular/core';
import { TreoMediaWatcherService } from '@treo/services/media-watcher/media-watcher.service';

@NgModule({
    providers: [TreoMediaWatcherService],
})
export class TreoMediaWatcherModule {
    private readonly _treoMediaWatcherService = inject(TreoMediaWatcherService);
}
