package ordering

import (
	"context"
	"math"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) ValidateCart(ctx context.Context, lines []domain.CartLineInput) (domain.CartValidation, error) {
	ctx, span := tracer.Start(ctx, "service.ordering.ValidateCart")
	defer span.End()

	merged, err := mergeLines(lines)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.CartValidation{}, err
	}
	ids := make([]int64, len(merged))
	for i, l := range merged {
		ids[i] = l.ProductID
	}
	products, err := s.carts.LoadProducts(ctx, ids, false)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.CartValidation{}, err
	}

	out := domain.CartValidation{Groups: []domain.CartGroup{}}
	groupAt := map[int64]int{} // store id (0 for products that do not exist) -> index in out.Groups
	for _, l := range merged {
		p, found := products[l.ProductID]
		line, err := s.line(l, p, found)
		if err != nil {
			tracer.RecordError(span, err)
			return domain.CartValidation{}, err
		}
		storeID := int64(0)
		if found {
			storeID = p.Store.ID
		}
		gi, ok := groupAt[storeID]
		if !ok {
			gi = len(out.Groups)
			groupAt[storeID] = gi
			group := domain.CartGroup{Lines: []domain.ValidatedLine{}}
			if found {
				group.Seller = s.seller(p.Store)
			}
			out.Groups = append(out.Groups, group)
		}
		out.Groups[gi].Lines = append(out.Groups[gi].Lines, line)
		if line.Issue == nil {
			if out.Total > math.MaxInt64-line.LineTotal {
				err := invalid("cart total overflows")
				tracer.RecordError(span, err)
				return domain.CartValidation{}, err
			}
			out.Total += line.LineTotal
		}
	}
	return out, nil
}

// mergeLines validates the request and merges duplicate product ids, keeping first-seen order.
func mergeLines(lines []domain.CartLineInput) ([]domain.CartLineInput, error) {
	if len(lines) == 0 {
		return nil, invalid("lines must not be empty")
	}
	if len(lines) > MaxCartLines {
		return nil, invalid("a cart has at most 50 lines")
	}
	at := make(map[int64]int, len(lines))
	merged := make([]domain.CartLineInput, 0, len(lines))
	for _, l := range lines {
		if l.ProductID <= 0 {
			return nil, invalid("productId must be positive")
		}
		if l.Quantity < 1 {
			return nil, invalid("quantity must be at least 1")
		}
		if i, ok := at[l.ProductID]; ok {
			if merged[i].Quantity > math.MaxInt32-l.Quantity {
				return nil, invalid("quantity is too large")
			}
			merged[i].Quantity += l.Quantity
			continue
		}
		at[l.ProductID] = len(merged)
		merged = append(merged, l)
	}
	return merged, nil
}

// line classifies one merged request line. A product that does not exist is a bare unavailable line.
func (s *service) line(l domain.CartLineInput, p domain.CartProduct, found bool) (domain.ValidatedLine, error) {
	v := domain.ValidatedLine{ProductID: l.ProductID, Quantity: l.Quantity}
	if !found {
		unavailable := domain.IssueProductUnavailable
		v.Issue = &unavailable
		return v, nil
	}
	total, err := domain.LineTotal(p.Price, l.Quantity)
	if err != nil {
		return domain.ValidatedLine{}, err
	}
	v.Name, v.UnitPrice, v.LineTotal = p.Name, p.Price, total
	if p.Image != nil {
		v.Image = s.imageSet(*p.Image)
	}
	var issue domain.LineIssue
	switch {
	case !p.Orderable():
		issue = domain.IssueProductUnavailable
	case p.Stock == 0 || p.Status == domain.ProductOutOfStock:
		issue = domain.IssueOutOfStock
		v.AvailableStock = p.Stock
	case p.Stock < l.Quantity:
		issue = domain.IssueInsufficientStock
		v.AvailableStock = p.Stock
	default:
		v.AvailableStock = p.Stock
	}
	if issue != "" {
		v.Issue = &issue
	}
	return v, nil
}

func (s *service) seller(st domain.CartStore) domain.SellerSummary {
	out := domain.SellerSummary{
		ID: st.ID, Name: st.Name, Location: st.Location, Rating: st.Rating, RatingCount: st.RatingCount,
		PickupAddress: st.PickupAddress, OpeningHours: st.Hours.Display(), WhatsApp: st.WhatsApp,
	}
	if st.Avatar != nil {
		img := s.imageSet(*st.Avatar)
		out.Avatar = &img
	}
	return out
}
