package handler

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// TestUptimeKumaPush sends a test push to the configured Uptime Kuma endpoint
func TestUptimeKumaPush(c *gin.Context) {
	appConfig := c.MustGet("CONFIG").(config.Interface)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	// Load settings to get the push URL
	settings, err := deviceRepo.LoadSettings(c)
	if err != nil {
		logger.Errorf("Failed to load settings for Uptime Kuma test: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"errors":  []string{"Failed to load settings"},
		})
		return
	}

	// Determine push URL: config file takes precedence, then settings DB
	pushURL := appConfig.GetString("web.uptime_kuma.push_url")
	if pushURL == "" && settings != nil {
		pushURL = settings.Metrics.UptimeKumaPushURL
	}

	if pushURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"errors":  []string{"Uptime Kuma push URL is not configured"},
		})
		return
	}

	// Build test push URL with query parameters
	u, err := url.Parse(pushURL)
	if err != nil {
		logger.Errorf("Invalid Uptime Kuma push URL: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"errors":  []string{fmt.Sprintf("Invalid push URL: %v", err)},
		})
		return
	}

	q := u.Query()
	q.Set("status", "up")
	q.Set("msg", "Test from Scrutiny")
	q.Set("ping", "0")
	u.RawQuery = q.Encode()

	// Send test push
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	if appConfig.GetBool("web.uptime_kuma.insecure_skip_verify") {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	resp, err := client.Get(u.String())
	if err != nil {
		logger.Errorf("Uptime Kuma test push failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"errors":  []string{fmt.Sprintf("Push request failed: %v", err)},
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Errorf("Uptime Kuma test push returned HTTP %d", resp.StatusCode)
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"errors":  []string{fmt.Sprintf("Push returned HTTP %d", resp.StatusCode)},
		})
		return
	}

	logger.Info("Uptime Kuma test push sent successfully")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
