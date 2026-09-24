// Package leader elects one Scrutiny replica to run background jobs (monitors and the report
// scheduler) when several web replicas share one database (#880). A single instance always
// wins the election, so behavior without replicas is unchanged.
package leader

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// SchedulerLeaseName is the lease that gates monitors and the report scheduler.
	SchedulerLeaseName = "scheduler"

	// DefaultTTL is how long a lease lives without renewal. A replica that stops renewing is
	// replaced within this window.
	// ponytail: expiry uses each replica's own clock, so clock skew between replicas must stay well
	// under the TTL; switch to database time if that ever stops holding.
	DefaultTTL = 30 * time.Second
)

// Store persists leases. database.DeviceRepo implements it.
type Store interface {
	TryAcquireLease(ctx context.Context, name string, holder string, ttl time.Duration) (bool, error)
	ReleaseLease(ctx context.Context, name string, holder string) error
}

// Elector holds or waits for one named lease and renews it in the background.
type Elector struct {
	store  Store
	logger logrus.FieldLogger
	name   string
	holder string
	ttl    time.Duration

	held   atomic.Bool
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New returns an elector for the named lease. Call Start to begin campaigning.
func New(store Store, logger logrus.FieldLogger, name string, ttl time.Duration) *Elector {
	return &Elector{
		store:  store,
		logger: logger,
		name:   name,
		holder: newHolderID(),
		ttl:    ttl,
	}
}

// Held reports whether this replica currently owns the lease.
func (e *Elector) Held() bool {
	return e.held.Load()
}

// Holder returns this replica's holder ID.
func (e *Elector) Holder() string {
	return e.holder
}

// Start makes one acquisition attempt immediately, then renews at a third of the TTL until Stop.
func (e *Elector) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.campaign(ctx)

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(e.ttl / 3)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.campaign(ctx)
			}
		}
	}()
}

// Stop ends renewal and releases the lease if held, so another replica can take over at once.
func (e *Elector) Stop() {
	if e.cancel == nil {
		return
	}
	e.cancel()
	e.wg.Wait()
	if e.held.Swap(false) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.store.ReleaseLease(ctx, e.name, e.holder); err != nil {
			e.logger.Warnf("Failed to release %q lease: %v", e.name, err)
		}
	}
}

func (e *Elector) campaign(ctx context.Context) {
	ok, err := e.store.TryAcquireLease(ctx, e.name, e.holder, e.ttl)
	if err != nil {
		// Without a confirmed renewal this replica cannot know it still leads, so it stops acting.
		ok = false
		if ctx.Err() == nil {
			e.logger.Warnf("Failed to renew %q lease: %v", e.name, err)
		}
	}
	if was := e.held.Swap(ok); was != ok {
		if ok {
			e.logger.Infof("Acquired %q lease as %s; this replica runs background jobs", e.name, e.holder)
		} else {
			e.logger.Infof("Lost %q lease; another replica runs background jobs", e.name)
		}
	}
}

func newHolderID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%d-%s", host, os.Getpid(), hex.EncodeToString(b))
}
