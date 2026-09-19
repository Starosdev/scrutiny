import { DeviceSelfTestModel } from 'app/core/models/device-selftest-model';

export interface DeviceSelfTestHealthModel {
    status: 'unknown' | 'passed' | 'failed';
    has_result: boolean;
    latest_observed_at: number | null;
    has_failures: boolean;
}

export interface DeviceSelfTestsResponseWrapper {
    success: boolean;
    errors?: any[];
    data: {
        self_tests: DeviceSelfTestModel[];
        health: DeviceSelfTestHealthModel;
    };
}
