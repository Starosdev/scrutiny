import { btrfsUsagePercent } from './btrfs-filesystem-model';

describe('btrfsUsagePercent', () => {
    it('uses used / (used + free_statfs), matching df', () => {
        // Real synology2 volume: 89.7% of raw device size, but 93% by df
        const pct = btrfsUsagePercent({ used: 48409019846656, device_size: 53948010463232, free_statfs: 3485342298112, free_min: 2867116863488 });
        expect(pct).toBe(93.3);
    });

    it('falls back to free_min when free_statfs is missing', () => {
        expect(btrfsUsagePercent({ used: 90, device_size: 1000, free_statfs: 0, free_min: 10 })).toBe(90);
    });

    it('falls back to device size when no free figure exists', () => {
        expect(btrfsUsagePercent({ used: 500, device_size: 1000, free_statfs: 0, free_min: 0 })).toBe(50);
        expect(btrfsUsagePercent({ used: 0, device_size: 0, free_statfs: 0, free_min: 0 })).toBe(0);
    });
});
