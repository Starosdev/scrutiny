package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func UpdateDeviceLabel(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	wwn := c.Param("wwn")
	if err := validation.ValidateWWN(wwn); err != nil {
		logger.Warnf("Invalid WWN format: %s", wwn)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	var request struct {
		Label string `json:"label"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Errorln("Invalid request body", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request"})
		return
	}

	err := deviceRepo.UpdateDeviceLabel(c, wwn, request.Label)
	if err != nil {
		logger.Errorln("An error occurred while updating device label", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
