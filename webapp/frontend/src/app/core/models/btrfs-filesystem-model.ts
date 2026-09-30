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

/**
 * Usage percentage computed the way df / NAS UIs do: used / (used + available).
 * Dividing by raw device size instead counts unallocated space that Btrfs can't fully hand out
 * (DUP metadata, reserves), which under-reports how full the filesystem is.
 * Falls back to the conservative `free_min` estimate, then to device size, for older data.
 */
export function btrfsUsagePercent(fs: { used: number; device_size: number; free_statfs: number; free_min?: number }): number {
    const free = fs.free_statfs > 0 ? fs.free_statfs : (fs.free_min ?? 0);
    const total = free > 0 ? fs.used + free : fs.device_size;
    if (total <= 0) {
        return 0;
    }
    return Number(((fs.used / total) * 100).toFixed(1));
}
