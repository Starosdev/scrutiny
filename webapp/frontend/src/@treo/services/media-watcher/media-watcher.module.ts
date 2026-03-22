import { NgModule } from '@angular/core';
import { TreoMediaWatcherService } from '@treo/services/media-watcher/media-watcher.service';

@NgModule({
    providers: [
        TreoMediaWatcherService
    ]
})
export class TreoMediaWatcherModule
{
    /**
     * Constructor
     *
     * @param {TreoMediaWatcherService} _treoMediaWatcherService
     */
    constructor(
        private readonly _treoMediaWatcherService: TreoMediaWatcherService
    )
    {
    }
}
