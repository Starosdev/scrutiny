import { Injectable, inject } from '@angular/core';
import { ActivatedRouteSnapshot, Router, RouterStateSnapshot } from '@angular/router';
import { Observable, of } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { BtrfsFilesystemDetailService } from 'app/modules/btrfs-filesystem-detail/btrfs-filesystem-detail.service';
import { BtrfsFilesystemDetailsResponseWrapper } from 'app/core/models/btrfs-filesystem-summary-model';

@Injectable({ providedIn: 'root' })
export class BtrfsFilesystemDetailResolver {
    private readonly _btrfsFilesystemDetailService = inject(BtrfsFilesystemDetailService);
    private readonly _router = inject(Router);

    resolve(route: ActivatedRouteSnapshot, _state: RouterStateSnapshot): Observable<BtrfsFilesystemDetailsResponseWrapper> {
        return this._btrfsFilesystemDetailService.getData(route.params.uuid).pipe(
            catchError((error) => {
                console.error('Failed to load Btrfs filesystem details:', error);
                this._router.navigate(['/btrfs-filesystems']);
                return of(null);
            })
        );
    }
}
