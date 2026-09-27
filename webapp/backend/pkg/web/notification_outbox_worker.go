package web

import (
	"context"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/sirupsen/logrus"
)

// outboxPollInterval bounds how long a notification recorded by another replica waits for the
// leader. Rows recorded by the leader itself are delivered at once through the gate's wake signal.
const outboxPollInterval = 5 * time.Second

// NotificationOutboxWorker delivers outbox notifications while this replica is leader (#880).
type NotificationOutboxWorker struct {
	appEngine *AppEngine
	logger    logrus.FieldLogger
	repo      database.DeviceRepo

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewNotificationOutboxWorker(ae *AppEngine, repo database.DeviceRepo) *NotificationOutboxWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &NotificationOutboxWorker{appEngine: ae, logger: ae.Logger, repo: repo, ctx: ctx, cancel: cancel}
}

func (w *NotificationOutboxWorker) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(outboxPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
			case <-w.appEngine.NotificationGate.Wake():
			}
			if w.appEngine.isLeader() {
				w.drain()
			}
		}
	}()
}

func (w *NotificationOutboxWorker) Stop() {
	w.cancel()
	w.wg.Wait()
}

func (w *NotificationOutboxWorker) drain() {
	settings, err := w.repo.LoadSettings(w.ctx)
	if err != nil || settings == nil {
		w.logger.Warnf("Notification outbox: failed to load settings: %v", err)
		return
	}
	if err := w.appEngine.NotificationGate.DrainOutbox(w.ctx, w.appEngine.Config, settings); err != nil && w.ctx.Err() == nil {
		w.logger.Warnf("Notification outbox: delivery failed: %v", err)
	}
}
