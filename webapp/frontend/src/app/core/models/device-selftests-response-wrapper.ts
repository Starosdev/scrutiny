import { DeviceSelfTestModel } from 'app/core/models/device-selftest-model';

export interface DeviceSelfTestsResponseWrapper {
    success: boolean;
    errors?: any[];
    data: {
        self_tests: DeviceSelfTestModel[];
    };
}
