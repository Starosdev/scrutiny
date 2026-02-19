package handler

import (
	"context"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/reports"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ReportScheduler interface to avoid import cycle with web package
type ReportScheduler interface {
	GenerateOnDemand(ctx context.Context, periodType string) (*reports.ReportData, error)
	GenerateOnDemandPDF(ctx context.Context, periodType string) (string, error)
}

func GenerateReport(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	scheduler := c.MustGet("REPORT_SCHEDULER").(ReportScheduler)

	format := c.DefaultQuery("format", "text")
	period := c.DefaultQuery("period", "daily")

	if period != "daily" && period != "weekly" && period != "monthly" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid period: must be daily, weekly, or monthly"})
		return
	}

	if format == "pdf" {
		pdfPath, err := scheduler.GenerateOnDemandPDF(c.Request.Context(), period)
		if err != nil {
			logger.Errorf("Failed to generate PDF report: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.File(pdfPath)
		return
	}

	report, err := scheduler.GenerateOnDemand(c.Request.Context(), period)
	if err != nil {
		logger.Errorf("Failed to generate report: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	subject, message := reports.FormatTextReport(report)

	if c.DefaultQuery("test", "") == "true" {
		logger.Info("Test report requested, sending via notification system")
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"subject": subject,
		"message": message,
		"data":    report,
	})
}
