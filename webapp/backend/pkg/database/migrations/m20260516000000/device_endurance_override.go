package m20260516000000

type DeviceEnduranceOverride struct {
	WWN    string  `json:"wwn" gorm:"primaryKey"`
	MaxTBW float64 `json:"max_tbw"`
}

func (DeviceEnduranceOverride) TableName() string {
	return "device_endurance_overrides"
}
