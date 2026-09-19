import { DEFAULT_TEMPERATURE_THRESHOLD_CELSIUS, DEFAULT_TEMPERATURE_DURATION_MINUTES } from 'app/core/config/app.config';

export const settings = {
    zfs_pool_modifications_allowed: true,
    settings: {
        theme: 'light',
        layout: 'material',
        dashboard_display: 'name',
        dashboard_sort: 'status',
        dashboard_columns: 2,
        dashboard_density: 'comfortable',
        temperature_unit: 'celsius',
        file_size_si_units: false,
        powered_on_hours_unit: 'humanize',
        time_format: '24',
        line_stroke: 'smooth',
        metrics: {
            notify_on_temperature: false,
            temperature_threshold_celsius: DEFAULT_TEMPERATURE_THRESHOLD_CELSIUS,
            temperature_duration_minutes: DEFAULT_TEMPERATURE_DURATION_MINUTES,
            notify_level: 2,
            status_filter_attributes: 0,
            status_threshold: 3,
            repeat_notifications: true,
            consumer_drive_profiles_enabled: true,
            consumer_drive_profiles_denylist: '',
        },
    },
};
