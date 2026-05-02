package models

import (
	"time"
)

// MDADMArray represents an MDADM RAID array in the database
type MDADMArray struct {
	// GORM attributes
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`

	// Array identifier (UUID) - primary key
	UUID  string `json:"uuid" gorm:"primary_key"`
	Name  string `json:"name"`
	Level string `json:"level"`

	// Member devices (stored as JSON string in SQLite)
	Devices []string `json:"devices" gorm:"type:text;serializer:json"`

	// User provided metadata
	Label string `json:"label,omitempty"`

	// Management flags
	Archived bool `json:"archived"`
	Muted    bool `json:"muted"`
}

// MDADMArrayWrapper wraps the response for MDADM array API calls
type MDADMArrayWrapper struct {
	Success bool         `json:"success"`
	Errors  []error      `json:"errors,omitempty"`
	Data    []MDADMArray `json:"data"`
}
