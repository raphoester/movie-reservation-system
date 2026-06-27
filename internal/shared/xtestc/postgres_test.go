package xtestc_test

import (
	"context"
	"testing"
	"time"

	"github.com/raphoester/movie-reservation-system/internal/shared/xid"
	"github.com/raphoester/movie-reservation-system/internal/shared/xpg"
	"github.com/raphoester/movie-reservation-system/internal/shared/xtestc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgres(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pg := xtestc.NewPostgres("example_db")

	t.Cleanup(func() {
		pg.TearDownSuiteBg()
	})

	err := pg.Bootstrap(ctx)
	require.NoError(t, err)

	count1, err := countExamples(ctx, pg)
	require.NoError(t, err)
	assert.EqualValues(t, 0, count1)

	err = insertExample(ctx, pg)
	require.NoError(t, err)

	count2, err := countExamples(ctx, pg)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count2)

	pg.SetupTest(t)

	count3, err := countExamples(ctx, pg)
	require.NoError(t, err)
	assert.EqualValues(t, 0, count3)
}

func TestPostgres_CreateDatabase(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pg := xtestc.NewPostgres("example_db")

	t.Cleanup(func() {
		pg.TearDownSuiteBg()
	})

	err := pg.Bootstrap(ctx)
	require.NoError(t, err)

	err = pg.Client().CreateDatabase(ctx, "new_example_db")
	require.NoError(t, err, "failed to create new database")
}

func TestPostgres_CreateDatabase_AlreadyExists(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pg := xtestc.NewPostgres("example_db")

	t.Cleanup(func() {
		pg.TearDownSuiteBg()
	})

	err := pg.Bootstrap(ctx)
	require.NoError(t, err)

	err = pg.Client().CreateDatabase(ctx, "example_db_2")
	require.NoError(t, err)

	err = pg.Client().CreateDatabase(ctx, "example_db_2")
	require.Error(t, err)
	assert.ErrorIs(t, err, xpg.ErrDatabaseAlreadyExists)
}

func TestPostgres_PoolConfig_MaxOpenConns_IsApplied(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	pg := xtestc.NewPostgres("example_db")
	t.Cleanup(func() { pg.TearDownSuiteBg() })

	err := pg.Bootstrap(ctx)
	require.NoError(t, err)

	maxOpen := 7
	client, err := pg.NewClientWithPool(xpg.PoolConfig{MaxOpenConns: &maxOpen})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	assert.Equal(t, maxOpen, client.SQLClient().Stats().MaxOpenConnections)
}

func TestPostgres_PoolConfig_DefaultsPreserved_WhenNilPool(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	pg := xtestc.NewPostgres("example_db")
	t.Cleanup(func() { pg.TearDownSuiteBg() })

	err := pg.Bootstrap(ctx)
	require.NoError(t, err)

	client, err := pg.NewClientWithPool(xpg.PoolConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	// Go default: 0 means unlimited open connections.
	assert.Equal(t, 0, client.SQLClient().Stats().MaxOpenConnections)
}

func TestPostgres_SetupTest_TruncatesNonPublicSchemas(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	pg := xtestc.NewPostgres("example_db")
	t.Cleanup(func() { pg.TearDownSuiteBg() })

	err := pg.Bootstrap(ctx)
	require.NoError(t, err)

	_, err = pg.Client().ExecContext(ctx,
		`INSERT INTO example_schema.example_table (name) VALUES ('test')`)
	require.NoError(t, err)

	pg.SetupTest(t)

	var count int64
	res := pg.Client().GormClient().WithContext(ctx).
		Table("example_schema.example_table").
		Count(&count)
	require.NoError(t, res.Error)
	assert.EqualValues(t, 0, count)
}

func countExamples(ctx context.Context, pg *xtestc.Postgres) (int64, error) {
	var count int64
	res := pg.Client().
		GormClient().
		WithContext(ctx).
		Table("example_table").
		Count(&count)
	return count, res.Error
}

func insertExample(ctx context.Context, pg *xtestc.Postgres) error {
	type Example struct {
		ID   string
		Name string
	}

	example := Example{
		ID:   xid.GetTestStringID(),
		Name: "Test Name",
	}

	res := pg.Client().
		GormClient().
		WithContext(ctx).
		Table("example_table").
		Create(&example)

	return res.Error
}
