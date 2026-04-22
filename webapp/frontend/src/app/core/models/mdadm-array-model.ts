export interface MDADMArrayModel {
    uuid: string;
    name: string;
    level: string;
    devices: string[];
    label?: string;
    archived: boolean;
    muted: boolean;
    created_at: string;
    updated_at: string;
    // Latest metrics (populated by summary endpoint)
    state?: string;
    sync_progress?: number;
}

export interface MDADMMetricsHistoryModel {
    date: string;
    active_devices: number;
    working_devices: number;
    failed_devices: number;
    spare_devices: number;
    state: string;
    sync_progress: number;
    raw_mdstat?: string;
}

export interface MDADMArrayResponseWrapper {
    success: boolean;
    data: MDADMArrayModel[];
    errors?: string[];
}

export interface MDADMArrayDetailResponseWrapper {
    success: boolean;
    data: {
        array: MDADMArrayModel;
        history: MDADMMetricsHistoryModel[];
        latest_metrics: MDADMMetricsHistoryModel;
    };
    errors?: string[];
}
