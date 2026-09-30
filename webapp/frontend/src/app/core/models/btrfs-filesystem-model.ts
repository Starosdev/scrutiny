export interface BtrfsFilesystemModel {
    uuid: string;
    host_id: string;
    label: string;
    archived: boolean;
    muted: boolean;
    status: BtrfsFilesystemStatus;
    mount_point: string;

    device_count: number;
    device_size: number;
    device_allocated: number;
    device_unallocated: number;
    device_missing: number;
    used: number;
    free_estimated: number;
    free_min: number;
    free_statfs: number;
    statfs_used: number;
    statfs_available: number;
    data_ratio: number;
    metadata_ratio: number;
    multiple_profiles: boolean;
    data_profile: string;
    metadata_profile: string;
    system_profile: string;
    data_total: number;
    data_used: number;
    metadata_total: number;
    metadata_used: number;
    system_total: number;
    system_used: number;

    scrub_state: BtrfsScrubState;
    scrub_started_at?: string;
    scrub_finished_at?: string;
    scrub_duration?: string;
    scrub_total_bytes: number;
    scrub_scrubbed_bytes: number;
    scrub_error_summary: string;
    scrub_read_errors: number;
    scrub_csum_errors: number;
    scrub_verify_errors: number;
    scrub_super_errors: number;

    devices?: BtrfsDeviceModel[];

    created_at: string;
    updated_at: string;
}

export type BtrfsFilesystemStatus = 'ONLINE' | 'DEGRADED';
export type BtrfsScrubState = 'unknown' | 'idle' | 'running' | 'finished' | 'aborted';

export interface BtrfsDeviceModel {
    row_id: number;
    filesystem_uuid: string;
    id: number;
    path: string;
    size: number;
    missing: boolean;
    read_io_errors: number;
    write_io_errors: number;
    flush_io_errors: number;
    corruption_errors: number;
    generation_errors: number;
}

type BtrfsCapacityFields = Pick<BtrfsFilesystemModel, 'used' | 'device_size' | 'statfs_used' | 'statfs_available'>;

/**
 * Used and total bytes the way df and NAS UIs report them, from the collector's statfs(2) figures.
 * Both count one copy of the data, so this is right for every profile (single, DUP, RAID1, mixed).
 * btrfs `used` / `device_size` instead count raw space, which reads low because unallocated space
 * can't all become file space. That is only the fallback, for data from collectors without statfs.
 */
export function btrfsCapacity(fs: BtrfsCapacityFields): { used: number; total: number } {
    if (fs.statfs_used + fs.statfs_available > 0) {
        return { used: fs.statfs_used, total: fs.statfs_used + fs.statfs_available };
    }
    return { used: fs.used, total: fs.device_size };
}

export function btrfsUsagePercent(fs: BtrfsCapacityFields): number {
    const { used, total } = btrfsCapacity(fs);
    if (total <= 0) {
        return 0;
    }
    return Number(((used / total) * 100).toFixed(1));
}
