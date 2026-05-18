package common

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type SmartSupport struct {
	Enabled   *bool `json:"enabled,omitempty"`
	Available bool  `json:"available"`
}

func (s *SmartSupport) UnmarshalJSON(data []byte) error {
	if s == nil {
		return fmt.Errorf("cannot unmarshal smart_support into nil receiver")
	}
	return s.scanBytes(data)
}

func (s SmartSupport) Value() (driver.Value, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(payload), nil
}

func (s *SmartSupport) Scan(value interface{}) error {
	if s == nil {
		return fmt.Errorf("cannot scan smart_support into nil receiver")
	}
	if value == nil {
		*s = SmartSupport{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return s.scanBytes(v)
	case string:
		return s.scanBytes([]byte(v))
	case bool:
		*s = SmartSupport{Available: v}
		return nil
	case int64:
		*s = SmartSupport{Available: v != 0}
		return nil
	case float64:
		*s = SmartSupport{Available: v != 0}
		return nil
	default:
		return fmt.Errorf("unsupported smart_support scan type %T", value)
	}
}

func (s *SmartSupport) scanBytes(raw []byte) error {
	if len(raw) == 0 {
		*s = SmartSupport{}
		return nil
	}

	type smartSupportAlias SmartSupport
	var object smartSupportAlias
	if err := json.Unmarshal(raw, &object); err == nil {
		*s = SmartSupport(object)
		return nil
	}

	var legacyBool bool
	if err := json.Unmarshal(raw, &legacyBool); err == nil {
		*s = SmartSupport{Available: legacyBool}
		return nil
	}

	var legacyNumber float64
	if err := json.Unmarshal(raw, &legacyNumber); err == nil {
		*s = SmartSupport{Available: legacyNumber != 0}
		return nil
	}

	return fmt.Errorf("unsupported smart_support payload: %s", string(raw))
}

func (SmartSupport) GormDataType() string {
	return "json"
}

func (SmartSupport) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Name() {
	case "postgres":
		return "JSONB"
	case "sqlite":
		return "TEXT"
	default:
		return "JSON"
	}
}
