package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	collector_models "github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RegisterMdadmArrays registers detected MDADM arrays from a collector
func RegisterMdadmArrays(c *gin.Context) {
	dbRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	var collectorWrapper struct {
		Data []collector_models.MDADMArray `json:"data"`
	}

	if err := c.ShouldBindJSON(&collectorWrapper); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "errors": []string{err.Error()}})
		return
	}

	var registeredArrays []models.MDADMArray
	var registrationErrors []string
	for _, collectorArray := range collectorWrapper.Data {
		trimmedUUID := strings.TrimSpace(collectorArray.UUID)
		if trimmedUUID == "" {
			registrationErrors = append(registrationErrors, fmt.Sprintf("array %s rejected: missing UUID", collectorArray.Name))
			continue
		}

		array := models.MDADMArray{
			UUID:    trimmedUUID,
			Name:    collectorArray.Name,
			Level:   collectorArray.Level,
			Devices: collectorArray.Devices,
			HostID:  collectorArray.HostID,
		}

		if err := dbRepo.RegisterMdadmArray(c.Request.Context(), array); err != nil {
			logger.Errorf("Failed to register MDADM array %s: %v", array.UUID, err)
			registrationErrors = append(registrationErrors, fmt.Sprintf("array %s (%s) registration failed: %v", array.Name, array.UUID, err))
			continue
		}
		registeredArrays = append(registeredArrays, array)
	}

	c.JSON(http.StatusOK, models.MDADMArrayWrapper{
		Success: len(registeredArrays) > 0,
		Errors:  registrationErrors,
		Data:    registeredArrays,
	})
}
