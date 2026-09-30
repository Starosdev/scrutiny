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

/** One copy's worth of used bytes, in the same units as free_statfs (btrfs `used` counts every copy). */
export function btrfsLogicalUsed(fs: Pick<BtrfsFilesystemModel, 'data_used' | 'metadata_used' | 'system_used'>): number {
    return fs.data_used + fs.metadata_used + fs.system_used;
}

/**
 * Usage percentage computed the way df and NAS UIs do: logical used / (logical used + free_statfs).
 * Both figures count one copy, so this is right for mirrored profiles too. Dividing raw `used` by
 * raw device size instead counts unallocated space btrfs can't fully hand out, and reads low.
 * Falls back to used / device_size when free_statfs is unknown (older collectors or history).
 */
export function btrfsUsagePercent(logicalUsed: number, freeStatfs: number, used: number, deviceSize: number): number {
    if (logicalUsed > 0 && freeStatfs > 0) {
        return Number(((logicalUsed / (logicalUsed + freeStatfs)) * 100).toFixed(1));
    }
    if (deviceSize <= 0) {
        return 0;
    }
    return Number(((used / deviceSize) * 100).toFixed(1));
}
