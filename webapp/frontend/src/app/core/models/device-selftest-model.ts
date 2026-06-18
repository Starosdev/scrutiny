export interface DeviceSelfTestModel {
    id: number;
    created_at: string;
    updated_at: string;
    device_id: string;
    device_wwn: string;
    type_value: number;
    type_string: string;
    status_value: number;
    status_string: string;
    status_passed: boolean;
    lifetime_hours: number;
}
