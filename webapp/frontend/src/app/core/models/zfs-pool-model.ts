// maps to webapp/backend/pkg/models/zfs_pool.go
export interface ZFSPoolModel {
    guid: string;
    name: string;
    host_id: string;
    label: string;
    archived: boolean;
    muted: boolean;

    status: ZFSPoolStatus;
    size: number;
    allocated: number;
    free: number;
    fragmentation: number;
    capacity_percent: number;

    // usable_used and usable_free come from the pool's root dataset and exclude
    // parity, unlike size/allocated/free which are raw vdev capacity. Both are 0
    // for pools last reported by a collector that predates them.
    usable_used: number;
    usable_free: number;

    scrub_state: ZFSScrubState;
    scrub_start_time?: string;
    scrub_end_time?: string;
    scrub_scanned_bytes: number;
    scrub_issued_bytes: number;
    scrub_total_bytes: number;
    scrub_errors_count: number;
    scrub_percent_complete: number;

    total_read_errors: number;
    total_write_errors: number;
    total_checksum_errors: number;

    vdevs?: ZFSVdevModel[];

    created_at: string;
    updated_at: string;
    last_seen_at?: string;
    last_inventory_at?: string;
    presence?: ZFSPoolPresence;
}

/**
 * Post-parity capacity the pool can actually store, from its root dataset.
 * Returns 0 when the pool was last reported by a collector that predates
 * usable capacity, which callers treat as "fall back to the raw size".
 */
export function zfsUsableSize(pool: ZFSPoolModel): number {
    return (pool.usable_used || 0) + (pool.usable_free || 0);
}

export type ZFSPoolStatus = 'ONLINE' | 'DEGRADED' | 'FAULTED' | 'OFFLINE' | 'REMOVED' | 'UNAVAIL';
export type ZFSPoolPresence = 'present' | 'missing' | 'stale' | 'unknown';
export type ZFSScrubState = 'none' | 'scanning' | 'finished' | 'canceled';

export interface ZFSVdevModel {
    id: number;
    pool_guid: string;
    parent_id?: number;

    name: string;
    type: ZFSVdevType;
    status: string;
    path: string;

    read_errors: number;
    write_errors: number;
    checksum_errors: number;

    children?: ZFSVdevModel[];

    created_at: string;
    updated_at: string;
}

export type ZFSVdevType = 'disk' | 'file' | 'mirror' | 'raidz1' | 'raidz2' | 'raidz3' | 'spare' | 'log' | 'cache' | 'special' | 'dedup';
