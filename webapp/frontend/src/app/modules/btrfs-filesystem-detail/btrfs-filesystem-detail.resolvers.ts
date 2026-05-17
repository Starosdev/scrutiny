import { Injectable } from '@angular/core';
import { ActivatedRouteSnapshot, Router, RouterStateSnapshot } from '@angular/router';
import { Observable, of } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { BtrfsFilesystemDetailService } from 'app/modules/btrfs-filesystem-detail/btrfs-filesystem-detail.service';
import { BtrfsFilesystemDetailsResponseWrapper } from 'app/core/models/btrfs-filesystem-summary-model';

@Injectable({ providedIn: 'root' })
export class BtrfsFilesystemDetailResolver {
    constructor(private readonly _btrfsFilesystemDetailService: BtrfsFilesystemDetailService, private readonly _router: Router) {}

    resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<BtrfsFilesystemDetailsResponseWrapper> {
        return this._btrfsFilesystemDetailService.getData(route.params.uuid).pipe(
            catchError((error) => {
                console.error('Failed to load Btrfs filesystem details:', error);
                this._router.navigate(['/btrfs-filesystems']);
                return of(null);
            })
        );
    }
}
