package database

import (
	"context"
	"errors"
	"net/http"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/golang/mock/gomock"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	influxhttp "github.com/influxdata/influxdb-client-go/v2/api/http"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// stubBucketsAPI records what the repository asked for and replays a scripted InfluxDB.
// Only the three methods the bucket code calls are implemented; a call to any other method
// panics rather than returning a wrong answer quietly.
type stubBucketsAPI struct {
	influxapi.BucketsAPI

	// findsFailUntil makes the first N FindBucketByName calls report a missing bucket, which is
	// how a caller enters the create path.
	findsFailUntil int
	// foundOrgID is the organization of the bucket a successful find returns. Empty means the
	// organization the create was addressed to.
	foundOrgID string
	createErr  error

	finds        int
	creates      int
	createdRules []domain.RetentionRule
	updated      *domain.Bucket
}

var _ influxapi.BucketsAPI = (*stubBucketsAPI)(nil)

func (s *stubBucketsAPI) FindBucketByName(_ context.Context, bucketName string) (*domain.Bucket, error) {
	s.finds++
	if s.finds <= s.findsFailUntil {
		return nil, errors.New("bucket '" + bucketName + "' not found")
	}
	bucket := &domain.Bucket{Name: bucketName}
	if s.foundOrgID != "" {
		orgID := s.foundOrgID
		bucket.OrgID = &orgID
	}
	return bucket, nil
}

func (s *stubBucketsAPI) CreateBucketWithName(_ context.Context, org *domain.Organization, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error) {
	s.creates++
	s.createdRules = rules
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &domain.Bucket{Name: bucketName, OrgID: org.Id}, nil
}

func (s *stubBucketsAPI) UpdateBucket(_ context.Context, bucket *domain.Bucket) (*domain.Bucket, error) {
	s.updated = bucket
	return bucket, nil
}

// stubInfluxClient exposes a scripted BucketsAPI so the repository's own bucket code can be
// exercised. Only BucketsAPI is implemented, for the same reason as above.
type stubInfluxClient struct {
	influxdb2.Client
	buckets influxapi.BucketsAPI
}

var _ influxdb2.Client = (*stubInfluxClient)(nil)

func (s *stubInfluxClient) BucketsAPI() influxapi.BucketsAPI { return s.buckets }

func influxConflict(bucketName string) error {
	return &influxhttp.Error{
		StatusCode: http.StatusConflict,
		Code:       "conflict",
		Message:    "bucket with name " + bucketName + " already exists",
	}
}

func testOrg() *domain.Organization {
	orgID := "org-scrutiny"
	return &domain.Organization{Name: "scrutiny", Id: &orgID}
}

// fixes #1004: the Test Backend job gives pkg/web and pkg/database one InfluxDB container, both
// ran EnsureBuckets at once, and the process that lost the create race failed the whole suite.
func TestEnsureBucketExistsTreatsConflictAsSuccess(t *testing.T) {
	buckets := &stubBucketsAPI{createErr: influxConflict("metrics_yearly"), foundOrgID: "org-scrutiny"}

	bucket, err := ensureBucketExists(context.Background(), buckets, testOrg(), "metrics_yearly")

	require.NoError(t, err)
	require.NotNil(t, bucket)
	require.Equal(t, "metrics_yearly", bucket.Name)
	require.Equal(t, 1, buckets.creates)
}

// only a 409 means the bucket is already there. Any other failure has to surface, because a
// create can fail for reasons the re-read cannot see.
func TestEnsureBucketExistsReturnsNonConflictCreateError(t *testing.T) {
	buckets := &stubBucketsAPI{
		createErr:  &influxhttp.Error{StatusCode: http.StatusUnauthorized, Code: "unauthorized", Message: "insufficient permissions"},
		foundOrgID: "org-scrutiny",
	}

	bucket, err := ensureBucketExists(context.Background(), buckets, testOrg(), "metrics_yearly")

	require.ErrorContains(t, err, "unauthorized")
	require.Nil(t, bucket)
}

// FindBucketByName queries by name across every organization the token can read, while bucket
// names are unique per organization. Another organization's bucket is not this one.
func TestEnsureBucketExistsRejectsBucketFromAnotherOrg(t *testing.T) {
	buckets := &stubBucketsAPI{createErr: influxConflict("metrics_yearly"), foundOrgID: "org-other"}

	bucket, err := ensureBucketExists(context.Background(), buckets, testOrg(), "metrics_yearly")

	require.ErrorContains(t, err, "conflict")
	require.Nil(t, bucket)
}

// a conflict with no bucket behind it is a real error and still has to surface.
func TestEnsureBucketExistsReturnsErrorWhenBucketIsStillMissing(t *testing.T) {
	buckets := &stubBucketsAPI{createErr: influxConflict("metrics_yearly"), findsFailUntil: 99}

	bucket, err := ensureBucketExists(context.Background(), buckets, testOrg(), "metrics_yearly")

	require.ErrorContains(t, err, "conflict")
	require.Nil(t, bucket)
}

// the repository's own call path: the first find misses, the create loses the race, and the
// retention rule this process is configured with still has to reach the bucket. Without the
// fall-through the loser silently inherits whatever rule the winner created the bucket with.
func TestEnsureRetentionBucketAppliesRetentionAfterLosingCreateRace(t *testing.T) {
	buckets := &stubBucketsAPI{
		createErr:      influxConflict("metrics"),
		findsFailUntil: 1,
		foundOrgID:     "org-scrutiny",
	}
	repo := &scrutinyRepository{influxClient: &stubInfluxClient{buckets: buckets}}
	rule := domain.RetentionRule{EverySeconds: 1234}

	err := repo.ensureRetentionBucket(context.Background(), testOrg(), "metrics", rule, true)

	require.NoError(t, err)
	require.Equal(t, []domain.RetentionRule{rule}, buckets.createdRules)
	require.NotNil(t, buckets.updated)
	require.Equal(t, domain.RetentionRules{rule}, buckets.updated.RetentionRules)
}

// the yearly bucket keeps its infinite retention: it is created with no rules at all.
func TestEnsureBucketsCreatesYearlyBucketWithoutRetentionRules(t *testing.T) {
	buckets := &stubBucketsAPI{findsFailUntil: 99}
	repo := &scrutinyRepository{
		appConfig:    newBucketTestConfig(t),
		logger:       logrus.New(),
		influxClient: &stubInfluxClient{buckets: buckets},
	}

	require.NoError(t, repo.EnsureBuckets(context.Background(), testOrg()))

	require.Equal(t, 4, buckets.creates)
	require.Empty(t, buckets.createdRules)
}

func newBucketTestConfig(t *testing.T) *mock_config.MockInterface {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetBool(gomock.Any()).Return(false).AnyTimes()
	fakeConfig.EXPECT().GetInt(gomock.Any()).Return(0).AnyTimes()
	fakeConfig.EXPECT().GetString(gomock.Any()).Return("metrics").AnyTimes()
	return fakeConfig
}
