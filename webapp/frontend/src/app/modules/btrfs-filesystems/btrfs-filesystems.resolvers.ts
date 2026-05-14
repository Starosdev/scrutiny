import { Injectable } from '@angular/core';
import { ActivatedRouteSnapshot, RouterStateSnapshot } from '@angular/router';
import { Observable } from 'rxjs';
import { BtrfsFilesystemsService } from 'app/modules/btrfs-filesystems/btrfs-filesystems.service';
import { BtrfsFilesystemModel } from 'app/core/models/btrfs-filesystem-model';

@Injectable({ providedIn: 'root' })
export class BtrfsFilesystemsResolver {
    constructor(private readonly _btrfsFilesystemsService: BtrfsFilesystemsService) {}

    resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<Record<string, BtrfsFilesystemModel>> {
        return this._btrfsFilesystemsService.getSummaryData();
    }
}
