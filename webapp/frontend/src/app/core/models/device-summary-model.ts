import {DeviceModel} from 'app/core/models/device-model';
import {SmartTemperatureModel} from 'app/core/models/measurements/smart-temperature-model';

// maps to webapp/backend/pkg/models/device_summary.go
export interface DeviceSummaryModel {
    device: DeviceModel;
    smart?: SmartSummary;
    temp_history?: SmartTemperatureModel[];
}

export interface SmartSummary {
    collector_date?: string,
    temp?: number
    power_on_hours?: number
    // SSD Health Metrics (only present for SSDs)
    // percentage_used: NVMe percentage_used or ATA devstat_7_8 (0-100%, higher = more worn)
    percentage_used?: number
    // wearout_value: ATA attributes 177, 233, 231, 232 (0-100%, higher = healthier)
    wearout_value?: number
}

