export const mdadm_summary = {
    success: true,
    data: [
        {
            uuid: '6d4b0f0d-8f5b-4af0-a8ac-9efeb7d3832e',
            name: 'md0',
            level: 'raid1',
            devices: ['/dev/sda1', '/dev/sdb1'],
            label: 'Primary Mirror',
            archived: false,
            muted: false,
            created_at: '2026-05-15T00:00:00Z',
            updated_at: '2026-05-15T00:00:00Z',
            state: 'clean',
            sync_progress: 100,
            array_size: 499963174912,
            used_bytes: 268435456000
        },
        {
            uuid: '0dcb7b9c-4d96-4d9b-9e62-18d81ef7cb75',
            name: 'md1',
            level: 'raid5',
            devices: ['/dev/sdc1', '/dev/sdd1', '/dev/sde1'],
            archived: false,
            muted: false,
            created_at: '2026-05-15T00:00:00Z',
            updated_at: '2026-05-15T00:00:00Z',
            state: 'clean, degraded, recovering',
            sync_progress: 42.5,
            array_size: 1999852699648,
            used_bytes: 1099511627776
        }
    ]
};
