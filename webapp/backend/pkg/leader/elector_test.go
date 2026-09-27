package leader

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// fakeStore holds one lease in memory with the same semantics as the database upsert.
type fakeStore struct {
	mu       sync.Mutex
	holder   string
	expires  time.Time
	fail     bool
	released []string
}

func (s *fakeStore) TryAcquireLease(_ context.Context, _ string, holder string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return false, errors.New("database unavailable")
	}
	now := time.Now()
	if s.holder == "" || s.holder == holder || s.expires.Before(now) {
		s.holder, s.expires = holder, now.Add(ttl)
		return true, nil
	}
	return false, nil
}

func (s *fakeStore) ReleaseLease(_ context.Context, _ string, holder string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.released = append(s.released, holder)
	if s.holder == holder {
		s.holder = ""
	}
	return nil
}

func (s *fakeStore) setFail(fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = fail
}

func TestElector_OnlyOneReplicaLeads(t *testing.T) {
	store := &fakeStore{}
	a := New(store, logrus.New(), SchedulerLeaseName, time.Minute)
	b := New(store, logrus.New(), SchedulerLeaseName, time.Minute)
	a.Start()
	b.Start()
	defer b.Stop()

	require.True(t, a.Held())
	require.False(t, b.Held())

	a.Stop()
	require.False(t, a.Held())
	require.Equal(t, []string{a.Holder()}, store.released)

	b.campaign(context.Background())
	require.True(t, b.Held(), "a released lease passes to the waiting replica")
}

func TestElector_StepsDownWhenRenewalFails(t *testing.T) {
	store := &fakeStore{}
	e := New(store, logrus.New(), SchedulerLeaseName, 30*time.Millisecond)
	e.Start()
	defer e.Stop()
	require.True(t, e.Held())

	store.setFail(true)
	require.Eventually(t, func() bool { return !e.Held() }, time.Second, 5*time.Millisecond)

	store.setFail(false)
	require.Eventually(t, e.Held, time.Second, 5*time.Millisecond)
}

func TestElector_StopWithoutStart(t *testing.T) {
	New(&fakeStore{}, logrus.New(), SchedulerLeaseName, time.Minute).Stop()
}
