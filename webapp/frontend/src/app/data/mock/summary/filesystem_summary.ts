import { FilesystemSummaryResponseWrapper } from 'app/core/models/filesystem-summary-model';

export const filesystem_summary: FilesystemSummaryResponseWrapper = {
    success: true,
    data: {
        filesystems: {
            atlas: [
                {
                    host_id: 'atlas',
                    mount_point: '/',
                    source_device: '/dev/sda1',
                    filesystem_type: 'ext4',
                    total_bytes: 1000000000,
                    used_bytes: 700000000,
                    available_bytes: 300000000,
                    used_percent: 70,
                    updated_at: '2026-05-10T00:00:00Z'
                }
            ]
        },
        hosts: {
            atlas: {
                host_id: 'atlas',
                status: 'available',
                filesystem_count: 1,
                updated_at: '2026-05-10T00:00:00Z'
            }
        }
    }
};
