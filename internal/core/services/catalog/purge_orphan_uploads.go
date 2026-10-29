package catalog

import (
	"context"

	log "github.com/oemahdev/logger"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

// PurgeOrphanUploads deletes the rows first, then the objects: a failed object delete leaks a file
// (logged) but never leaves a row pointing at a missing object.
func (s *service) PurgeOrphanUploads(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(ctx, "service.catalog.PurgeOrphanUploads")
	defer span.End()

	orphans, err := s.uploads.DeleteOrphans(ctx, s.clock.Now().Add(-s.cfg.OrphanAge))
	if err != nil {
		tracer.RecordError(span, err)
		return 0, err
	}
	for _, keys := range orphans {
		for _, k := range keys.All() {
			if err := s.storage.Delete(ctx, k); err != nil {
				tracer.RecordError(span, err)
				log.ErrorCtx(ctx, "[service.catalog.PurgeOrphanUploads] delete object failed", log.Field("error", err))
			}
		}
	}
	if len(orphans) > 0 {
		log.InfoCtx(ctx, "orphan uploads purged", log.Field("count", len(orphans)))
	}
	return len(orphans), nil
}
