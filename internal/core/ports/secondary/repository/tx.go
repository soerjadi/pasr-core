package repository

import "context"

// TxManager runs fn in a transaction. Repositories called with the ctx passed to fn
// join that transaction; an error or panic rolls it back.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
