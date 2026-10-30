package repository

import "context"

type IdempotencyRepository interface {
	// Claim inserts the (userID, key) row with requestHash unless it exists. claimed is true when this
	// call inserted it; otherwise existingOrderID and existingHash describe the stored row. Inside a
	// transaction a row inserted but not yet committed by another transaction makes Claim wait for it:
	// after a commit Claim reports that row, after a rollback it claims the key itself.
	Claim(ctx context.Context, userID int64, key, requestHash string) (existingOrderID int64, claimed bool, existingHash string, err error)
	// Attach records the order created for a claimed key.
	Attach(ctx context.Context, userID int64, key string, orderID int64) error
}
