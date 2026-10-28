import { DeviceModel } from 'app/core/models/device-model';
import { SmartModel } from 'app/core/models/measurements/smart-model';
import { AttributeMetadataModel } from 'app/core/models/thresholds/attribute-metadata-model';
import { FilesystemCapacityModel, FilesystemHostStatusModel } from 'app/core/models/filesystem-summary-model';

// maps to webapp/backend/pkg/models/device_summary.go
export interface DeviceDetailsResponseWrapper {
    success: boolean;
    errors?: any[];
    data: {
        device: DeviceModel;
        smart_results: SmartModel[];
        filesystems?: FilesystemCapacityModel[];
        filesystem_status?: FilesystemHostStatusModel;
        // True when the filesystem collector has reported for any host, which
        // is what separates "never enabled" from "running under a host id this
        // drive's host does not share".
        filesystem_collector_reporting?: boolean;
    };
    metadata: { [key: string]: AttributeMetadataModel } | { [key: number]: AttributeMetadataModel };
}
