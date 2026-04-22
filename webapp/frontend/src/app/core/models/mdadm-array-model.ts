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
}

export interface MDADMMetricsHistoryModel {
    date: string;
    active_devices: number;
    working_devices: number;
    failed_devices: number;
    spare_devices: number;
    state: string;
    sync_progress: number;
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
    };
    errors?: string[];
}
