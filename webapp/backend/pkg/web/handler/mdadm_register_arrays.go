package handler

import (
	"net/http"

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
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "errors": []error{err}})
			return
		}

		var registeredArrays []models.MDADMArray
		for _, collectorArray := range collectorWrapper.Data {
			array := models.MDADMArray{
				UUID:    collectorArray.UUID,
				Name:    collectorArray.Name,
				Level:   collectorArray.Level,
				Devices: collectorArray.Devices,
			}

			if err := dbRepo.RegisterMdadmArray(c.Request.Context(), array); err != nil {
				logger.Errorf("Failed to register MDADM array %s: %v", array.UUID, err)
				continue
			}
			registeredArrays = append(registeredArrays, array)
		}

		c.JSON(http.StatusOK, models.MDADMArrayWrapper{
			Success: true,
			Data:    registeredArrays,
		})
	}
