package handler

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// stateNotification describes a subsystem upload (Btrfs, ZFS) whose problems
// are found by comparing the upload with the row stored by the previous one.
type stateNotification struct {
	// failureTypes reports the problems that warrant a notification for the
	// given repeat setting, ordered by severity; empty means nothing to send.
	failureTypes func(repeat bool) []string
	// build creates the notification for a non-empty failureTypes result.
	build      func(appConfig config.Interface, failureTypes []string) notify.Notify
	kind       string
	identifier string
	// silenced is true when the stored row is muted or archived. Those flags
	// are written by their own routes and never carried by the upload payload.
	silenced bool
}

// notifyStateChange sends at most one notification for an upload, through the
// shared gate when one is registered. A healthy upload never loads settings.
func notifyStateChange(c *gin.Context, deviceRepo database.DeviceRepo, logger *logrus.Entry, n stateNotification) {
	if n.silenced {
		logger.Debugf("Skipping notification for %s %s: muted or archived", n.kind, n.identifier)
		return
	}
	// Repeat on is the widest filter: nothing it rejects passes without it.
	if len(n.failureTypes(true)) == 0 {
		return
	}

	settings, err := deviceRepo.LoadSettings(c.Request.Context())
	if err != nil {
		logger.Warnf("Failed to load settings for %s notification: %v", n.kind, err)
	}
	repeat := settings != nil && settings.Metrics.RepeatNotifications
	failureTypes := n.failureTypes(repeat)
	if len(failureTypes) == 0 {
		return
	}

	appConfig := c.MustGet("CONFIG").(config.Interface)
	notification := n.build(appConfig, failureTypes)
	notification.LoadDatabaseUrls(c.Request.Context(), deviceRepo)
	sendNotificationWithGate(c, logger, n.identifier, &notification, settings)
}
