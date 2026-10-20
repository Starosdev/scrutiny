package database

import (
	"context"
	"errors"
	"testing"

	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/stretchr/testify/require"
)

// conflictBucketsAPI returns the 409 InfluxDB sends when another process created the bucket
// between this caller's FindBucketByName and its CreateBucketWithName. bucketExists controls
// whether the re-read after the conflict finds the bucket.
type conflictBucketsAPI struct {
	influxapi.BucketsAPI
	bucketExists bool
	creates      int
}

func (c *conflictBucketsAPI) CreateBucketWithName(_ context.Context, _ *domain.Organization, bucketName string, _ ...domain.RetentionRule) (*domain.Bucket, error) {
	c.creates++
	return nil, errors.New("conflict: bucket with name " + bucketName + " already exists")
}

func (c *conflictBucketsAPI) FindBucketByName(_ context.Context, bucketName string) (*domain.Bucket, error) {
	if !c.bucketExists {
		return nil, errors.New("bucket not found")
	}
	name := bucketName
	return &domain.Bucket{Name: name}, nil
}

// fixes #1004: the Test Backend job gives pkg/web and pkg/database one InfluxDB container, both
// ran EnsureBuckets at once, and the process that lost the create race failed the whole suite.
func TestEnsureBucketExistsTreatsConflictAsSuccess(t *testing.T) {
	buckets := &conflictBucketsAPI{bucketExists: true}

	err := ensureBucketExists(context.Background(), buckets, &domain.Organization{Name: "scrutiny"}, "metrics_yearly")

	require.NoError(t, err)
	require.Equal(t, 1, buckets.creates)
}

// a create failure with no bucket behind it is a real error and still has to surface.
func TestEnsureBucketExistsReturnsErrorWhenBucketIsStillMissing(t *testing.T) {
	buckets := &conflictBucketsAPI{bucketExists: false}

	err := ensureBucketExists(context.Background(), buckets, &domain.Organization{Name: "scrutiny"}, "metrics_yearly")

	require.Error(t, err)
}
