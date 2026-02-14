// maps to webapp/backend/pkg/models/measurements/performance.go
export interface PerformanceModel {
    date: string;
    device_wwn: string;
    device_protocol: string;
    profile: string;

    seq_read_bw_bytes: number;
    seq_write_bw_bytes: number;
    rand_read_iops: number;
    rand_write_iops: number;
    rand_read_lat_ns_avg: number;
    rand_read_lat_ns_p50: number;
    rand_read_lat_ns_p95: number;
    rand_read_lat_ns_p99: number;
    rand_write_lat_ns_avg: number;
    rand_write_lat_ns_p50: number;
    rand_write_lat_ns_p95: number;
    rand_write_lat_ns_p99: number;
    mixed_rw_iops: number;
    fio_version: string;
    test_duration_sec: number;
}

export interface PerformanceBaselineModel {
    seq_read_bw_bytes: number;
    seq_write_bw_bytes: number;
    rand_read_iops: number;
    rand_write_iops: number;
    rand_read_lat_ns_avg: number;
    rand_write_lat_ns_avg: number;
    sample_count: number;
}

export interface PerformanceResponseWrapper {
    success: boolean;
    data: {
        history: PerformanceModel[];
        baseline: PerformanceBaselineModel;
    };
}
