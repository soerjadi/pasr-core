package catalog

import "github.com/oemah/nusantara/core/internal/core/domain"

func (s *service) productSummary(p domain.ProductRecord, images []domain.ImageKeys) domain.ProductSummary {
	var img domain.ImageSet
	if len(images) > 0 {
		img = s.imageSet(images[0])
	}
	return domain.ProductSummary{
		ID: p.ID, Name: p.Name, SellerID: p.SellerID, SellerName: p.SellerName, Price: p.Price, Image: img,
		Rating: p.Rating, RatingCount: p.RatingCount, Stock: p.Stock, HalalVerified: p.HalalVerified,
	}
}

func (s *service) sellerSummary(r domain.SellerRecord) domain.SellerSummary {
	out := domain.SellerSummary{
		ID: r.ID, Name: r.Name, Location: r.Location, Rating: r.Rating, RatingCount: r.RatingCount,
		PickupAddress: r.PickupAddress, OpeningHours: r.Hours.Display(), WhatsApp: r.WhatsApp,
	}
	if r.Avatar != nil {
		img := s.imageSet(*r.Avatar)
		out.Avatar = &img
	}
	return out
}

func notFound(what string) error {
	return domain.NewError(domain.ErrNotFound, "NOT_FOUND", what+" not found")
}
