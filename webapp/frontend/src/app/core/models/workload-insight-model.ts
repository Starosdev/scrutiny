export interface WorkloadInsightModel {
    device_wwn: string;
    device_protocol: string;

    // Device display fields (populated from Device record)
    device_name?: string;
    device_type?: string;
    model_name?: string;
    label?: string;
    device_label?: string;
    host_id?: string;
    device_serial_id?: string;
    device_uuid?: string;

    data_points: number;
    time_span_hours: number;
    daily_write_bytes: number;
    daily_read_bytes: number;
    total_write_bytes: number;
    total_read_bytes: number;
    read_write_ratio: number;
    intensity: string;
    endurance?: EnduranceEstimateModel;
    spike?: ActivitySpikeModel;
}

export interface EnduranceEstimateModel {
    available: boolean;
    percentage_used: number;
    estimated_lifespan_days?: number;
    tbw_so_far: number;
}

export interface ActivitySpikeModel {
    detected: boolean;
    recent_daily_write_bytes: number;
    baseline_daily_write_bytes: number;
    spike_factor: number;
    description: string;
}

export interface WorkloadResponseWrapper {
    success: boolean;
    data: {
        workload: Record<string, WorkloadInsightModel>;
    };
}
