// Package storage is the port for object storage of uploaded media.
package storage

import (
	"context"
	"io"
)

// ObjectStorage stores immutable objects addressed by key.
type ObjectStorage interface {
	// Put stores the reader's content under key.
	Put(ctx context.Context, key, contentType string, r io.Reader) error
	// Delete removes the object; deleting a missing object is not an error.
	Delete(ctx context.Context, key string) error
	// URL returns the public URL of the object.
	URL(key string) string
}
