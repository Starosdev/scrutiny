export interface FilesystemCapacityModel {
    host_id: string;
    mount_point: string;
    source_device: string;
    filesystem_type: string;
    total_bytes: number;
    used_bytes: number;
    available_bytes: number;
    used_percent: number;
    updated_at: string;
    // Whole disks backing this filesystem, by kernel device name. Absent when
    // the collector could not attribute it, which is not the same as a
    // filesystem with no disk behind it.
    backing_devices?: string[];
}

export interface FilesystemHostStatusModel {
    host_id: string;
    status: 'available' | 'unavailable';
    reason?: string;
    filesystem_count: number;
    updated_at: string;
}

export interface FilesystemSummaryResponseWrapper {
    success: boolean;
    data: {
        filesystems: Record<string, FilesystemCapacityModel[]>;
        hosts: Record<string, FilesystemHostStatusModel>;
    };
}
