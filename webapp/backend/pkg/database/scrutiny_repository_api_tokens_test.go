package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAPITokenTestRepo(t *testing.T) (*scrutinyRepository, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tokens.db")), &gorm.Config{})
	require.NoError(t, err)
	repo := &scrutinyRepository{gormClient: db, logger: logrus.New()}
	t.Cleanup(func() { _ = repo.Close() })
	require.NoError(t, repo.Migrate(ctx))
	return repo, ctx
}

func TestAPITokenLifecycle(t *testing.T) {
	repo, ctx := newAPITokenTestRepo(t)

	token := &models.ApiToken{Name: "phone", Scope: models.APITokenScopeRead, TokenHash: "hash-1"}
	require.NoError(t, repo.CreateAPIToken(ctx, token))
	require.NotZero(t, token.ID)
	require.False(t, token.Revoked)

	found, err := repo.GetActiveAPITokenByHash(ctx, "hash-1")
	require.NoError(t, err)
	require.Equal(t, token.ID, found.ID)
	require.Equal(t, models.APITokenScopeRead, found.Scope)

	_, err = repo.GetActiveAPITokenByHash(ctx, "other")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.NoError(t, repo.RevokeAPIToken(ctx, token.ID))
	_, err = repo.GetActiveAPITokenByHash(ctx, "hash-1")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "revoked token must not authenticate")

	// The record stays for audit and shows as revoked.
	listed, err := repo.ListAPITokens(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.True(t, listed[0].Revoked)

	require.ErrorIs(t, repo.RevokeAPIToken(ctx, 9999), gorm.ErrRecordNotFound)
}

func TestAPITokenExpiredIsRejected(t *testing.T) {
	repo, ctx := newAPITokenTestRepo(t)

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	require.NoError(t, repo.CreateAPIToken(ctx, &models.ApiToken{Name: "old", Scope: models.APITokenScopeRead, TokenHash: "expired", ExpiresAt: &past}))
	require.NoError(t, repo.CreateAPIToken(ctx, &models.ApiToken{Name: "new", Scope: models.APITokenScopeRead, TokenHash: "valid", ExpiresAt: &future}))

	_, err := repo.GetActiveAPITokenByHash(ctx, "expired")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = repo.GetActiveAPITokenByHash(ctx, "valid")
	require.NoError(t, err)
}

func TestTouchAPITokenThrottlesWrites(t *testing.T) {
	repo, ctx := newAPITokenTestRepo(t)

	token := &models.ApiToken{Name: "collector", Scope: models.APITokenScopeCollector, TokenHash: "hash-touch"}
	require.NoError(t, repo.CreateAPIToken(ctx, token))
	require.Nil(t, token.LastUsedAt)

	require.NoError(t, repo.TouchAPIToken(ctx, token.ID))
	first, err := repo.GetActiveAPITokenByHash(ctx, "hash-touch")
	require.NoError(t, err)
	require.NotNil(t, first.LastUsedAt)

	// A second touch within a minute leaves the stored value alone.
	require.NoError(t, repo.TouchAPIToken(ctx, token.ID))
	second, err := repo.GetActiveAPITokenByHash(ctx, "hash-touch")
	require.NoError(t, err)
	require.True(t, first.LastUsedAt.Equal(*second.LastUsedAt))
}
