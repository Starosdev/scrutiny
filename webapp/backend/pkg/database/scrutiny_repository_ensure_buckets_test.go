package database

import (
	"context"
	"errors"
	"fmt"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/golang/mock/gomock"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/stretchr/testify/require"
)

const testOrgID = "org-scrutiny"

// stubBucketsAPI records what the repository asked for and replays a scripted InfluxDB, keyed by
// bucket name so a single call to EnsureBuckets can script each of its four buckets separately.
// It implements only the three methods the bucket code calls; the embedded interface is nil, so
// any other call panics rather than returning a wrong answer quietly.
type stubBucketsAPI struct {
	influxapi.BucketsAPI

	// findsFailUntil makes the first N finds of a bucket report it missing, which is how a
	// caller enters the create path. A value of 1 is the race: the caller's own find misses and
	// the re-read after the failed create succeeds, because a concurrent creator got there.
	findsFailUntil map[string]int
	createErrs     map[string]error
	// foundOrgIDs overrides the organization of the bucket a find returns. Unset means the
	// organization the create was addressed to.
	foundOrgIDs map[string]string

	finds        map[string]int
	creates      map[string]int
	createdRules map[string][]domain.RetentionRule
	updated      map[string]domain.RetentionRules
	updatedIDs   []string
}

var _ influxapi.BucketsAPI = (*stubBucketsAPI)(nil)

func newStubBucketsAPI() *stubBucketsAPI {
	return &stubBucketsAPI{
		findsFailUntil: map[string]int{},
		createErrs:     map[string]error{},
		foundOrgIDs:    map[string]string{},
		finds:          map[string]int{},
		creates:        map[string]int{},
		createdRules:   map[string][]domain.RetentionRule{},
		updated:        map[string]domain.RetentionRules{},
	}
}

func (s *stubBucketsAPI) FindBucketByName(_ context.Context, bucketName string) (*domain.Bucket, error) {
	s.finds[bucketName]++
	if s.finds[bucketName] <= s.findsFailUntil[bucketName] {
		return nil, errors.New("bucket '" + bucketName + "' not found")
	}
	orgID := testOrgID
	if override, ok := s.foundOrgIDs[bucketName]; ok {
		orgID = override
	}
	id := "bucket-" + bucketName
	return &domain.Bucket{Id: &id, Name: bucketName, OrgID: &orgID}, nil
}

func (s *stubBucketsAPI) CreateBucketWithName(_ context.Context, org *domain.Organization, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error) {
	s.creates[bucketName]++
	s.createdRules[bucketName] = rules
	if err, ok := s.createErrs[bucketName]; ok {
		return nil, err
	}
	id := "bucket-" + bucketName
	return &domain.Bucket{Id: &id, Name: bucketName, OrgID: org.Id}, nil
}

func (s *stubBucketsAPI) UpdateBucket(_ context.Context, bucket *domain.Bucket) (*domain.Bucket, error) {
	// the real UpdateBucket dereferences the bucket ID, so do it here too: a caller that passes
	// a bucket without one has to fail the test rather than pass it
	s.updatedIDs = append(s.updatedIDs, *bucket.Id)
	s.updated[bucket.Name] = bucket.RetentionRules
	return bucket, nil
}

// stubInfluxClient exposes a scripted BucketsAPI so the repository's own bucket code runs against
// it. Only BucketsAPI is implemented, for the same reason as above.
type stubInfluxClient struct {
	influxdb2.Client
	buckets influxapi.BucketsAPI
}

var _ influxdb2.Client = (*stubInfluxClient)(nil)

func (s *stubInfluxClient) BucketsAPI() influxapi.BucketsAPI { return s.buckets }

// influxConflict is the error the InfluxDB client actually returns for a 409 on bucket creation.
// The generated REST client decodes a non-2xx body into fmt.Errorf("code: message") with no
// status code and no wrapped error, so a conflict is only ever a plain error with this text.
func influxConflict(bucketName string) error {
	return fmt.Errorf("%s: bucket with name %s already exists", domain.ErrorCodeConflict, bucketName)
}

func testOrg() *domain.Organization {
	orgID := testOrgID
	return &domain.Organization{Name: "scrutiny", Id: &orgID}
}

func newBucketTestRepository(buckets influxapi.BucketsAPI) *scrutinyRepository {
	return &scrutinyRepository{influxClient: &stubInfluxClient{buckets: buckets}}
}

// fixes #1004: the Test Backend job gives pkg/web and pkg/database one InfluxDB container, both
// ran EnsureBuckets at once, and the process that lost the create race failed the whole suite.
func TestEnsureBucketExistsTreatsConflictAsSuccess(t *testing.T) {
	buckets := newStubBucketsAPI()
	buckets.createErrs["metrics_yearly"] = influxConflict("metrics_yearly")

	bucket, err := ensureBucketExists(context.Background(), buckets, testOrg(), "metrics_yearly")

	require.NoError(t, err)
	require.NotNil(t, bucket)
	require.Equal(t, "metrics_yearly", bucket.Name)
	require.Equal(t, 1, buckets.creates["metrics_yearly"])
}

// FindBucketByName queries by name across every organization the token can read, while bucket
// names are unique per organization. Another organization's bucket is not this one, so the
// create error stands rather than being masked.
func TestEnsureBucketExistsRejectsBucketFromAnotherOrg(t *testing.T) {
	buckets := newStubBucketsAPI()
	buckets.createErrs["metrics_yearly"] = errors.New("unauthorized: insufficient permissions")
	buckets.foundOrgIDs["metrics_yearly"] = "org-other"

	bucket, err := ensureBucketExists(context.Background(), buckets, testOrg(), "metrics_yearly")

	require.ErrorContains(t, err, "unauthorized")
	require.Nil(t, bucket)
}

// a failed create with no bucket behind it is a real error and still has to surface.
func TestEnsureBucketExistsReturnsErrorWhenBucketIsStillMissing(t *testing.T) {
	buckets := newStubBucketsAPI()
	buckets.createErrs["metrics_yearly"] = influxConflict("metrics_yearly")
	buckets.findsFailUntil["metrics_yearly"] = 99

	bucket, err := ensureBucketExists(context.Background(), buckets, testOrg(), "metrics_yearly")

	require.ErrorContains(t, err, "conflict")
	require.Nil(t, bucket)
}

// the repository's own call path: the first find misses, the create loses the race, and the
// retention rule this process is configured with still has to reach the bucket. Without the
// fall-through the loser silently inherits whatever rule the winner created the bucket with.
func TestEnsureRetentionBucketAppliesRetentionAfterLosingCreateRace(t *testing.T) {
	buckets := newStubBucketsAPI()
	buckets.createErrs["metrics"] = influxConflict("metrics")
	buckets.findsFailUntil["metrics"] = 1
	rule := domain.RetentionRule{EverySeconds: 1234}

	err := newBucketTestRepository(buckets).ensureRetentionBucket(context.Background(), testOrg(), "metrics", rule, true)

	require.NoError(t, err)
	require.Equal(t, []domain.RetentionRule{rule}, buckets.createdRules["metrics"])
	require.Equal(t, domain.RetentionRules{rule}, buckets.updated["metrics"])
}

// the first start of a fresh install: the bucket does not exist, the create succeeds, and the
// retention rule still has to be applied. Production defaults web.influxdb.retention_policy to
// true, so this is the common path rather than an edge case.
func TestEnsureRetentionBucketAppliesRetentionAfterCreating(t *testing.T) {
	buckets := newStubBucketsAPI()
	buckets.findsFailUntil["metrics"] = 1
	rule := domain.RetentionRule{EverySeconds: 4321}

	err := newBucketTestRepository(buckets).ensureRetentionBucket(context.Background(), testOrg(), "metrics", rule, true)

	require.NoError(t, err)
	require.Equal(t, 1, buckets.creates["metrics"])
	require.Equal(t, []domain.RetentionRule{rule}, buckets.createdRules["metrics"])
	require.Equal(t, domain.RetentionRules{rule}, buckets.updated["metrics"])
	require.Equal(t, []string{"bucket-metrics"}, buckets.updatedIDs)
}

// the bucket and the traceback from #1004: EnsureBuckets has to survive losing the race on the
// yearly bucket, which does not go through ensureRetentionBucket.
func TestEnsureBucketsSurvivesConflictOnYearlyBucket(t *testing.T) {
	buckets := newStubBucketsAPI()
	buckets.createErrs["metrics_yearly"] = influxConflict("metrics_yearly")
	buckets.findsFailUntil["metrics_yearly"] = 1
	repo := newBucketTestRepository(buckets)
	repo.appConfig = newBucketTestConfig(t)

	require.NoError(t, repo.EnsureBuckets(context.Background(), testOrg()))

	require.Equal(t, 1, buckets.creates["metrics_yearly"])
}

// the yearly bucket keeps its infinite retention: it is created with no retention rules, while
// the down-sampling buckets are created with theirs.
func TestEnsureBucketsCreatesYearlyBucketWithoutRetentionRules(t *testing.T) {
	buckets := newStubBucketsAPI()
	for _, name := range []string{"metrics", "metrics_weekly", "metrics_monthly", "metrics_yearly"} {
		buckets.findsFailUntil[name] = 1
	}
	repo := newBucketTestRepository(buckets)
	repo.appConfig = newBucketTestConfig(t)

	require.NoError(t, repo.EnsureBuckets(context.Background(), testOrg()))

	require.Empty(t, buckets.createdRules["metrics_yearly"])
	require.Len(t, buckets.createdRules["metrics"], 1)
	require.Len(t, buckets.createdRules["metrics_weekly"], 1)
	require.Len(t, buckets.createdRules["metrics_monthly"], 1)
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
