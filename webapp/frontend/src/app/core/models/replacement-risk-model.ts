export interface AttributeContributionModel {
    attribute_id: string;
    display_name: string;
    weight: number;
    score: number;
    value: number;
    trend_score: number;
}

export type RiskCategory = 'healthy' | 'monitor' | 'plan_replacement' | 'replace_soon';

export interface ReplacementRiskModel {
    device_wwn: string;
    device_protocol: string;
    score: number;
    category: RiskCategory;
    contributions: AttributeContributionModel[];
    trend_window: string;
    trend_bonus: number;
    computed_at: string;
    consumer_drive_profiles_enabled: boolean;
    consumer_drive_profile_applied: boolean;
    consumer_drive_profile_family?: string;
}

export interface ReplacementRiskResponseWrapper {
    success: boolean;
    data: ReplacementRiskModel;
}
