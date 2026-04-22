package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetMdadmSummary returns a summary of all MDADM arrays
func GetMdadmSummary(c *gin.Context) {
	dbRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	arrays, err := dbRepo.GetMdadmArrays(c.Request.Context())
		if err != nil {
			logger.Errorf("Failed to get MDADM arrays summary: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "errors": []string{err.Error()}})
			return
		}

		c.JSON(http.StatusOK, models.MDADMArrayWrapper{
			Success: true,
			Data:    arrays,
		})
	}
