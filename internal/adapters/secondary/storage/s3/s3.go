// Package s3 stores objects in an S3-compatible bucket (AWS S3, Cloudflare R2, MinIO).
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/oemah/nusantara/core/internal/core/ports/secondary/storage"
)

type Config struct {
	Endpoint      string // host[:port] or full URL; an https scheme (or none) enables TLS
	Bucket        string
	AccessKey     string
	SecretKey     string
	PublicBaseURL string // public prefix the bucket is served from, e.g. a CDN or r2.dev domain
}

type Storage struct {
	client  *minio.Client
	bucket  string
	baseURL string
}

var _ storage.ObjectStorage = (*Storage)(nil)

func New(cfg Config) (*Storage, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, errors.New("s3 storage: endpoint and bucket are required")
	}
	host, secure := cfg.Endpoint, true
	if strings.Contains(cfg.Endpoint, "://") {
		u, err := url.Parse(cfg.Endpoint)
		if err != nil || u.Host == "" {
			return nil, errors.New("s3 storage: invalid endpoint")
		}
		host, secure = u.Host, u.Scheme == "https"
	}
	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: "auto", // R2 accepts "auto"; AWS callers can rely on bucket-region redirects
	})
	if err != nil {
		return nil, fmt.Errorf("s3 storage: client: %w", err)
	}
	return &Storage{client: client, bucket: cfg.Bucket, baseURL: strings.TrimRight(cfg.PublicBaseURL, "/")}, nil
}

func (s *Storage) Put(ctx context.Context, key, contentType string, r io.Reader) error {
	// A known size avoids the multipart upload path; readers without Len (streams) fall back to it.
	size := int64(-1)
	if l, ok := r.(interface{ Len() int }); ok {
		size = int64(l.Len())
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	})
	if err != nil {
		return fmt.Errorf("s3 storage: put object: %w", err)
	}
	return nil
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3 storage: delete object: %w", err)
	}
	return nil
}

func (s *Storage) URL(key string) string { return s.baseURL + "/" + key }
