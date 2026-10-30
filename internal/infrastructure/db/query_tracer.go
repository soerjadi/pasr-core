package db

import (
	"context"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/infrastructure/config"
)

type queryTruncatingTracer struct {
	next   pgx.QueryTracer
	maxLen int
}

func (t *queryTruncatingTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	data.SQL = truncateSQL(data.SQL, t.maxLen)
	return t.next.TraceQueryStart(ctx, conn, data)
}

func (t *queryTruncatingTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	t.next.TraceQueryEnd(ctx, conn, data)
}

func truncateSQL(sql string, maxLen int) string {
	if maxLen <= 0 || len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen]
}

// newQueryTracer returns nil when query tracing is disabled.
func newQueryTracer(cfg config.Database) pgx.QueryTracer {
	if !cfg.TraceEnabled {
		return nil
	}

	var opts []otelpgx.Option // trimmed SQL in span names is the otelpgx default
	if !cfg.TraceIncludeSQL {
		opts = append(opts, otelpgx.WithDisableSQLStatementInAttributes())
	}
	if cfg.TraceIncludeParams {
		opts = append(opts, otelpgx.WithIncludeQueryParameters())
	}

	tracer := otelpgx.NewTracer(opts...)
	if cfg.TraceMaxQueryLen > 0 {
		return &queryTruncatingTracer{next: tracer, maxLen: cfg.TraceMaxQueryLen}
	}
	return tracer
}
