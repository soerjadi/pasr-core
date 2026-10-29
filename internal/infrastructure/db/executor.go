package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
)

type txKey struct{}

// Executor implements repository.DB and repository.TxManager. Calls made with a context
// from WithTx run inside that transaction; all other calls use the pool.
type Executor struct {
	pool *pgxpool.Pool
}

var (
	_ repository.DB        = (*Executor)(nil)
	_ repository.TxManager = (*Executor)(nil)
)

func NewExecutor(pool *pgxpool.Pool) *Executor { return &Executor{pool: pool} }

func (e *Executor) runner(ctx context.Context) repository.DB {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return e.pool
}

func (e *Executor) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return e.runner(ctx).Exec(ctx, sql, args...)
}

func (e *Executor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return e.runner(ctx).QueryRow(ctx, sql, args...)
}

func (e *Executor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return e.runner(ctx).Query(ctx, sql, args...)
}

// WithTx runs fn in a transaction. A nested call joins the outer transaction.
func (e *Executor) WithTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db.WithTx begin: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		// Rollback must work even if ctx is cancelled or fn panicked.
		rbErr := tx.Rollback(context.WithoutCancel(ctx))
		if r := recover(); r != nil {
			panic(r)
		}
		if rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("db.WithTx rollback: %w", rbErr))
		}
	}()
	if err = fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db.WithTx commit: %w", err)
	}
	committed = true
	return nil
}
