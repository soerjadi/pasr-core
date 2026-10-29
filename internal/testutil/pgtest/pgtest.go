// Package pgtest provides a real PostgreSQL for integration tests: one container per test binary,
// migrated once, truncated before every test.
package pgtest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/oemah/nusantara/core/migrations"
)

var (
	once      sync.Once
	sharedURL string
	startErr  error
)

// keepTables are never truncated: goose bookkeeping and seeded reference data.
var keepTables = map[string]bool{"goose_db_version": true, "categories": true}

// URL returns the connection string of the shared, migrated database after truncating mutable tables.
// It skips the test under -short.
func URL(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: skipped under -short")
	}
	once.Do(func() { sharedURL, startErr = start() })
	if startErr != nil {
		t.Fatalf("pgtest: start postgres: %v", startErr)
	}
	reset(t)
	return sharedURL
}

// Pool returns a pool on the shared database, closed when the test ends.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := URL(t)
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgtest: connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func start() (string, error) {
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("nusantara"),
		tcpostgres.WithUsername("nusantara"),
		tcpostgres.WithPassword("nusantara"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		return "", err
	}
	// The container lives until the test binary exits; Ryuk reaps it.
	url, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return "", err
	}
	if err := migrate(ctx, url); err != nil {
		return "", err
	}
	return url, nil
}

func migrate(ctx context.Context, url string) error {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func reset(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, sharedURL)
	if err != nil {
		t.Fatalf("pgtest: connect for reset: %v", err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'public'`)
	if err != nil {
		t.Fatalf("pgtest: list tables: %v", err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			t.Fatalf("pgtest: scan table: %v", err)
		}
		if !keepTables[name] {
			tables = append(tables, pgIdent(name))
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("pgtest: list tables: %v", err)
	}
	if len(tables) == 0 {
		return
	}
	if _, err := pool.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("pgtest: truncate: %v", err)
	}
}

func pgIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
