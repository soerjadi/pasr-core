package catalog

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"

	log "github.com/oemahdev/logger"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) UploadImage(ctx context.Context, ownerID int64, r io.Reader) (domain.ImageUpload, error) {
	ctx, span := tracer.Start(ctx, "service.catalog.UploadImage")
	defer span.End()

	data, err := readBounded(r, s.cfg.MaxImageBytes)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.ImageUpload{}, err
	}

	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return domain.ImageUpload{}, ctx.Err()
	}
	variants, err := processImage(data, s.cfg.MaxImagePixels)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.ImageUpload{}, err
	}

	keys, err := newImageKeys()
	if err != nil {
		tracer.RecordError(span, err)
		return domain.ImageUpload{}, err
	}
	stored := make([]string, 0, 3)
	cleanup := func() {
		for _, k := range stored {
			if derr := s.storage.Delete(context.WithoutCancel(ctx), k); derr != nil {
				log.ErrorCtx(ctx, "[service.catalog.UploadImage] cleanup failed", log.Field("error", derr))
			}
		}
	}
	for i, k := range keys.All() {
		if err := s.storage.Put(ctx, k, "image/jpeg", bytes.NewReader(variants[i].data)); err != nil {
			cleanup()
			tracer.RecordError(span, err)
			return domain.ImageUpload{}, fmt.Errorf("catalog.UploadImage store: %w", err)
		}
		stored = append(stored, k)
	}

	id, err := s.uploads.Create(ctx, ownerID, keys)
	if err != nil {
		cleanup()
		tracer.RecordError(span, err)
		return domain.ImageUpload{}, err
	}
	return domain.ImageUpload{ID: id, Image: s.imageSet(keys)}, nil
}

func (s *service) imageSet(k domain.ImageKeys) domain.ImageSet {
	return domain.ImageSet{Small: s.storage.URL(k.Small), Medium: s.storage.URL(k.Medium), Large: s.storage.URL(k.Large)}
}

// readBounded reads at most max bytes; one more byte means the file is too large.
func readBounded(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		return nil, errors.New("catalog.UploadImage: max image bytes is not configured")
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, errImageTooBig
		}
		return nil, errImageInvalid
	}
	if int64(len(data)) > max {
		return nil, errImageTooBig
	}
	return data, nil
}

// newImageKeys returns unguessable keys: 128 random bits shared by the three variants.
func newImageKeys() (domain.ImageKeys, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return domain.ImageKeys{}, fmt.Errorf("catalog.newImageKeys: %w", err)
	}
	base := "images/" + hex.EncodeToString(b[:])
	return domain.ImageKeys{Small: base + "-s.jpg", Medium: base + "-m.jpg", Large: base + "-l.jpg"}, nil
}
