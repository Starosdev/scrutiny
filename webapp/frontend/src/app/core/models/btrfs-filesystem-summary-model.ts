import { BtrfsFilesystemModel } from './btrfs-filesystem-model';

export interface BtrfsFilesystemSummaryResponseWrapper {
    success: boolean;
    data: {
        filesystems: Record<string, BtrfsFilesystemModel>;
    };
}

export interface BtrfsFilesystemDetailsResponseWrapper {
    success: boolean;
    data: {
        filesystem: BtrfsFilesystemModel;
        metrics_history: BtrfsMetricsHistoryModel[];
    };
}

export interface BtrfsMetricsHistoryModel {
    date: string;
    device_size: number;
    device_allocated: number;
    device_unallocated: number;
    device_missing: number;
    used: number;
    free_estimated: number;
    free_statfs: number;
    data_ratio: number;
    metadata_ratio: number;
    status: string;
    scrub_state: string;
    scrub_read_errors: number;
    scrub_csum_errors: number;
    scrub_verify_errors: number;
    scrub_super_errors: number;
}
