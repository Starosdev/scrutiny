import { btrfsLogicalUsed, btrfsUsagePercent } from './btrfs-filesystem-model';

describe('btrfsUsagePercent', () => {
    it('matches df on a single-data / DUP-metadata volume', () => {
        // Real Synology volume: 89.7% by raw used / device size, 93% in DSM and df.
        const logical = btrfsLogicalUsed({ data_used: 48304729919488, metadata_used: 52144455680, system_used: 507904 });
        expect(btrfsUsagePercent(logical, 3485342298112, 48409019846656, 53948010463232)).toBe(93.3);
    });

    it('does not double-count mirrored data', () => {
        // Two 4TB disks in RAID1 holding 1TB: raw used is 2TB, statfs free is ~3TB. True usage is 25%.
        const tb = 1e12;
        expect(btrfsUsagePercent(1 * tb, 3 * tb, 2 * tb, 8 * tb)).toBe(25);
    });

    it('falls back to used / device size when free_statfs is unknown', () => {
        expect(btrfsUsagePercent(0, 0, 500, 1000)).toBe(50);
        expect(btrfsUsagePercent(400, 0, 500, 1000)).toBe(50);
        expect(btrfsUsagePercent(0, 0, 0, 0)).toBe(0);
    });
});
