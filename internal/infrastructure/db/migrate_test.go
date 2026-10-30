package db_test

import (
	"context"
	"io/fs"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/infrastructure/config"
	"github.com/oemah/nusantara/core/internal/infrastructure/db"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
	"github.com/oemah/nusantara/core/migrations"
)

// emptyDatabase creates a fresh database next to the shared test one and returns its URL.
func emptyDatabase(t *testing.T, name string) string {
	t.Helper()
	ctx := context.Background()
	base := pgtest.URL(t)
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`) })
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

func TestAutoMigrateAppliesEmbeddedMigrations(t *testing.T) {
	ctx := context.Background()
	pool, err := db.New(ctx, config.Database{URL: emptyDatabase(t, "automigrate_test"), MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	files, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("embedded migrations: %v %v", files, err)
	}
	for i := 0; i < 2; i++ { // the second run must be a no-op
		if err := db.Migrate(ctx, pool); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	var applied int
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT version_id) FROM goose_db_version WHERE version_id > 0 AND is_applied`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != len(files) {
		t.Fatalf("applied %d migrations, embedded %d", applied, len(files))
	}
	// the pool stays usable and the schema is there
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("users table after migrate: %v", err)
	}
}
