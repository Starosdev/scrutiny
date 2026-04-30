package m20260421000000

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"gorm.io/gorm"
)

// Migrate creates the mdadm_arrays table
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.MDADMArray{})
}
