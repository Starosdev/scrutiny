package database

import (
	"context"
	"fmt"
	"sort"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testOrgID   = "org-scrutiny"
	testOrgName = "scrutiny"
)

// stubBucketsAPI replays a scripted InfluxDB organization. It implements only the three methods
// the bucket code calls, and the embedded interface is nil on purpose: a call to anything else,
// including the cross-organization FindBucketByName this code deliberately avoids, panics rather
// than returning a wrong answer quietly.
type stubBucketsAPI struct {
	influxapi.BucketsAPI

	// present holds the buckets the organization has. A successful create adds to it.
	present map[string]bool
	// createdByRace names buckets a concurrent creator owns: the create fails, and the bucket
	// exists immediately afterwards, which is what losing the race looks like.
	createdByRace map[string]bool
	createErrs    map[string]error

	// pages, when set, is returned one page per call, which is how the stub stands in for the
	// API's paging without being able to read the opaque PagingOption values
	pages [][]domain.Bucket

	listedOrgs   []string
	creates      map[string]int
	createdRules map[string][]domain.RetentionRule
	updated      map[string]domain.RetentionRules
	updatedIDs   []string
}

var _ influxapi.BucketsAPI = (*stubBucketsAPI)(nil)

func newStubBucketsAPI(existing ...string) *stubBucketsAPI {
	stub := &stubBucketsAPI{
		present:       map[string]bool{},
		createdByRace: map[string]bool{},
		createErrs:    map[string]error{},
		creates:       map[string]int{},
		createdRules:  map[string][]domain.RetentionRule{},
		updated:       map[string]domain.RetentionRules{},
	}
	for _, name := range existing {
		stub.present[name] = true
	}
	return stub
}

func (s *stubBucketsAPI) FindBucketsByOrgName(_ context.Context, orgName string, _ ...influxapi.PagingOption) (*[]domain.Bucket, error) {
	s.listedOrgs = append(s.listedOrgs, orgName)

	if s.pages != nil {
		if len(s.pages) == 0 {
			empty := []domain.Bucket{}
			return &empty, nil
		}
		page := s.pages[0]
		s.pages = s.pages[1:]
		return &page, nil
	}

	names := make([]string, 0, len(s.present))
	for name := range s.present {
		names = append(names, name)
	}
	sort.Strings(names)

	buckets := make([]domain.Bucket, 0, len(names))
	for _, name := range names {
		id := bucketTestID(name)
		buckets = append(buckets, domain.Bucket{Id: &id, Name: name})
	}
	return &buckets, nil
}

func (s *stubBucketsAPI) CreateBucketWithName(_ context.Context, org *domain.Organization, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error) {
	s.creates[bucketName]++
	s.createdRules[bucketName] = rules
	if err, ok := s.createErrs[bucketName]; ok {
		// the winner of the race leaves the bucket behind
		if s.createdByRace[bucketName] {
			s.present[bucketName] = true
		}
		return nil, err
	}
	s.present[bucketName] = true
	id := bucketTestID(bucketName)
	return &domain.Bucket{Id: &id, Name: bucketName, OrgID: org.Id}, nil
}

func (s *stubBucketsAPI) UpdateBucket(_ context.Context, bucket *domain.Bucket) (*domain.Bucket, error) {
	// the real UpdateBucket dereferences the bucket ID, so do it here too: a caller that passes
	// a bucket without one has to fail the test rather than pass it
	s.updatedIDs = append(s.updatedIDs, *bucket.Id)
	s.updated[bucket.Name] = bucket.RetentionRules
	return bucket, nil
}

func bucketTestID(bucketName string) string { return "bucket-" + bucketName }

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
	return &domain.Organization{Name: testOrgName, Id: &orgID}
}

func newBucketTestRepository(t *testing.T, buckets influxapi.BucketsAPI, applyRetention bool) *scrutinyRepository {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString(cfgInfluxDBBucket).Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetBool(cfgInfluxDBRetentionPolicy).Return(applyRetention).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.influxdb.retention.daily").Return(testDailyRetention).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.influxdb.retention.weekly").Return(testWeeklyRetention).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.influxdb.retention.monthly").Return(testMonthlyRetention).AnyTimes()

	logger, _ := logrustest.NewNullLogger()
	return &scrutinyRepository{
		appConfig:    fakeConfig,
		logger:       logger,
		influxClient: &stubInfluxClient{buckets: buckets},
	}
}

// distinct values, so an assertion cannot pass on the wrong window's rule
const (
	testDailyRetention   = 111
	testWeeklyRetention  = 222
	testMonthlyRetention = 333
)

// fixes #1004: the Test Backend job gives pkg/web and pkg/database one InfluxDB container, both
// ran EnsureBuckets at once, and the process that lost the create race failed the whole suite.
func TestEnsureBucketExistsTreatsConflictAsSuccess(t *testing.T) {
	buckets := newStubBucketsAPI()
	buckets.createErrs["metrics_yearly"] = influxConflict("metrics_yearly")
	buckets.createdByRace["metrics_yearly"] = true

	bucket, err := ensureBucketExists(context.Background(), buckets, testOrg(), "metrics_yearly")

	require.NoError(t, err)
	require.NotNil(t, bucket)
	require.Equal(t, "metrics_yearly", bucket.Name)
	require.Equal(t, 1, buckets.creates["metrics_yearly"])
}

// a failed create with no bucket behind it in this organization is a real error and still has to
// surface. A bucket of the same name in another organization is not this bucket, and the
// organization-scoped lookup cannot see it.
func TestEnsureBucketExistsReturnsErrorWhenBucketIsStillMissing(t *testing.T) {
	buckets := newStubBucketsAPI()
	buckets.createErrs["metrics_yearly"] = influxConflict("metrics_yearly")

	bucket, err := ensureBucketExists(context.Background(), buckets, testOrg(), "metrics_yearly")

	require.ErrorContains(t, err, "conflict")
	require.ErrorContains(t, err, "not found in organization")
	require.Nil(t, bucket)
}

// every bucket lookup is scoped to this deployment's organization. The client's FindBucketByName
// queries by name across every organization the token can read and returns the first match, so
// using it could read, re-time and down-sample into another organization's bucket. The stub
// panics if it is called at all.
func TestEnsureBucketsLooksUpBucketsWithinTheOrg(t *testing.T) {
	buckets := newStubBucketsAPI("metrics", "metrics_weekly", "metrics_monthly", "metrics_yearly")
	repo := newBucketTestRepository(t, buckets, false)

	require.NoError(t, repo.EnsureBuckets(context.Background(), testOrg()))

	require.Equal(t, []string{testOrgName, testOrgName, testOrgName, testOrgName}, buckets.listedOrgs)
	require.Empty(t, buckets.creates)
}

// the repository's own call path: the bucket is missing, the create loses the race, and the
// retention rule this process is configured with still has to reach the bucket. Without the
// fall-through the loser silently inherits whatever rule the winner created the bucket with.
func TestEnsureRetentionBucketAppliesRetentionAfterLosingCreateRace(t *testing.T) {
	buckets := newStubBucketsAPI()
	buckets.createErrs["metrics"] = influxConflict("metrics")
	buckets.createdByRace["metrics"] = true
	rule := domain.RetentionRule{EverySeconds: 1234}

	err := newBucketTestRepository(t, buckets, true).ensureRetentionBucket(context.Background(), testOrg(), "metrics", rule, true)

	require.NoError(t, err)
	require.Equal(t, []domain.RetentionRule{rule}, buckets.createdRules["metrics"])
	require.Equal(t, domain.RetentionRules{rule}, buckets.updated["metrics"])
	require.Equal(t, []string{bucketTestID("metrics")}, buckets.updatedIDs)
}

// the first start of a fresh install: the bucket does not exist, the create succeeds, and the
// retention rule still has to be applied. Production defaults web.influxdb.retention_policy to
// true, so this is the common path rather than an edge case.
func TestEnsureRetentionBucketAppliesRetentionAfterCreating(t *testing.T) {
	buckets := newStubBucketsAPI()
	rule := domain.RetentionRule{EverySeconds: 4321}

	err := newBucketTestRepository(t, buckets, true).ensureRetentionBucket(context.Background(), testOrg(), "metrics", rule, true)

	require.NoError(t, err)
	require.Equal(t, 1, buckets.creates["metrics"])
	require.Equal(t, []domain.RetentionRule{rule}, buckets.createdRules["metrics"])
	require.Equal(t, domain.RetentionRules{rule}, buckets.updated["metrics"])
	require.Equal(t, []string{bucketTestID("metrics")}, buckets.updatedIDs)
}

// the bucket and the traceback from #1004: EnsureBuckets has to survive losing the race on the
// yearly bucket, which does not go through ensureRetentionBucket.
func TestEnsureBucketsSurvivesConflictOnYearlyBucket(t *testing.T) {
	buckets := newStubBucketsAPI("metrics", "metrics_weekly", "metrics_monthly")
	buckets.createErrs["metrics_yearly"] = influxConflict("metrics_yearly")
	buckets.createdByRace["metrics_yearly"] = true
	repo := newBucketTestRepository(t, buckets, false)

	require.NoError(t, repo.EnsureBuckets(context.Background(), testOrg()))

	require.Equal(t, 1, buckets.creates["metrics_yearly"])
}

// each bucket is created with its own configured retention period, and the yearly bucket with
// none at all, which is how it keeps an infinite retention.
func TestEnsureBucketsCreatesEachBucketWithItsOwnRetention(t *testing.T) {
	buckets := newStubBucketsAPI()
	repo := newBucketTestRepository(t, buckets, true)

	require.NoError(t, repo.EnsureBuckets(context.Background(), testOrg()))

	require.Equal(t, []domain.RetentionRule{{EverySeconds: testDailyRetention}}, buckets.createdRules["metrics"])
	require.Equal(t, []domain.RetentionRule{{EverySeconds: testWeeklyRetention}}, buckets.createdRules["metrics_weekly"])
	require.Equal(t, []domain.RetentionRule{{EverySeconds: testMonthlyRetention}}, buckets.createdRules["metrics_monthly"])
	require.Empty(t, buckets.createdRules["metrics_yearly"])
	require.Len(t, buckets.updatedIDs, 3)
}

// findBucketInOrg has to walk past the first page: the API returns 20 buckets per page by
// default and this code asks for 100, which a shared InfluxDB organization can exceed.
func TestFindBucketInOrgReadsPastTheFirstPage(t *testing.T) {
	first := make([]domain.Bucket, 0, bucketPageSize)
	for i := 0; i < bucketPageSize; i++ {
		name := fmt.Sprintf("unrelated_%d", i)
		id := bucketTestID(name)
		first = append(first, domain.Bucket{Id: &id, Name: name})
	}
	target := bucketTestID("metrics_yearly")
	second := []domain.Bucket{{Id: &target, Name: "metrics_yearly"}}

	buckets := newStubBucketsAPI()
	buckets.pages = [][]domain.Bucket{first, second}

	found, err := findBucketInOrg(context.Background(), buckets, testOrgName, "metrics_yearly")

	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "metrics_yearly", found.Name)
	require.Len(t, buckets.listedOrgs, 2)
}

// a bucket that is in no page is not found, and the walk stops on the short page rather than
// asking for the next one forever.
func TestFindBucketInOrgStopsOnAShortPage(t *testing.T) {
	buckets := newStubBucketsAPI()
	buckets.pages = [][]domain.Bucket{{}}

	found, err := findBucketInOrg(context.Background(), buckets, testOrgName, "metrics_yearly")

	require.ErrorContains(t, err, "not found in organization")
	require.Nil(t, found)
	require.Len(t, buckets.listedOrgs, 1)
}
