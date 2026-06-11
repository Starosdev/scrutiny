package m20260610000000

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"gorm.io/gorm"
)

// Migrate adds the host_id column to the mdadm_arrays table
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.MDADMArray{})
}
