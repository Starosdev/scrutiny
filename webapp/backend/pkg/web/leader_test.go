package web

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/leader"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestIsLeader(t *testing.T) {
	require.True(t, (&AppEngine{}).isLeader(), "without an elector the process runs background jobs")

	notStarted := leader.New(nil, logrus.New(), leader.SchedulerLeaseName, time.Minute)
	require.False(t, (&AppEngine{Leader: notStarted}).isLeader(), "an elector without the lease blocks background jobs")
}
