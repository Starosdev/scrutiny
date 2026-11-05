import { FilesystemCapacityModel } from 'app/core/models/filesystem-summary-model';

/**
 * Bar colour for a filesystem's used percentage. Shared by the dashboard
 * capacity panel and the device detail panel so one filesystem cannot be
 * green on one page and yellow on the other.
 */
export function filesystemUsageClass(filesystem: FilesystemCapacityModel): string {
    if (filesystem.used_percent >= 90) {
        return 'bg-red-500';
    }
    if (filesystem.used_percent >= 80) {
        return 'bg-yellow-500';
    }
    return 'bg-green-500';
}
