import { btrfsCapacity, btrfsUsagePercent } from './btrfs-filesystem-model';

const fields = (statfs_used: number, statfs_available: number, used: number, device_size: number) =>
    ({ statfs_used, statfs_available, used, device_size });

describe('btrfsUsagePercent', () => {
    it('matches df using statfs used / (used + available)', () => {
        // Real Synology volume: 89.7% by raw used / device size, 93% in DSM and df.
        expect(btrfsUsagePercent(fields(48304747749376, 3485342298112, 48409019846656, 53948010463232))).toBe(93.3);
    });

    it('does not double-count mirrored data', () => {
        // Two 4TB disks in RAID1 holding 1TB: statfs reports one copy, btrfs raw used is 2TB.
        const tb = 1e12;
        expect(btrfsUsagePercent(fields(1 * tb, 3 * tb, 2 * tb, 8 * tb))).toBe(25);
    });

    it('reports a full filesystem as 100%, not the raw fallback', () => {
        expect(btrfsUsagePercent(fields(900, 0, 950, 1000))).toBe(100);
    });

    it('falls back to used / device size when statfs is unknown', () => {
        expect(btrfsUsagePercent(fields(0, 0, 500, 1000))).toBe(50);
        expect(btrfsUsagePercent(fields(0, 0, 0, 0))).toBe(0);
    });
});

describe('btrfsCapacity', () => {
    it('returns the same figures the percentage uses', () => {
        expect(btrfsCapacity(fields(300, 100, 350, 1000))).toEqual({ used: 300, total: 400 });
        expect(btrfsCapacity(fields(0, 0, 350, 1000))).toEqual({ used: 350, total: 1000 });
    });
});
