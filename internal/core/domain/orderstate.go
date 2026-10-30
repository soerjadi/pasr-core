package domain

import (
	"fmt"
	"math"
	"time"
)

// Action is a state-machine event on a seller-order.
type Action string

const (
	ActionAccept   Action = "ACCEPT"
	ActionReject   Action = "REJECT"
	ActionReady    Action = "READY"
	ActionComplete Action = "COMPLETE"
	ActionCancel   Action = "CANCEL"
	ActionExpire   Action = "EXPIRE"
)

// Actor roles accepted by Transition.
const (
	ActorBuyer    = "buyer"
	ActorSeller   = "seller"
	ActorOperator = "operator"
	ActorSystem   = "system"
)

type transitionRule struct {
	to    OrderStatus
	from  []OrderStatus
	roles []string
}

var transitions = map[Action]transitionRule{
	ActionAccept:   {OrderSellerProcessing, []OrderStatus{OrderCreated}, []string{ActorSeller, ActorOperator}},
	ActionReject:   {OrderCancelled, []OrderStatus{OrderCreated}, []string{ActorSeller, ActorOperator}},
	ActionReady:    {OrderReadyForPickup, []OrderStatus{OrderSellerProcessing}, []string{ActorSeller, ActorOperator}},
	ActionComplete: {OrderCompleted, []OrderStatus{OrderReadyForPickup}, []string{ActorSeller, ActorOperator}},
	ActionCancel:   {OrderCancelled, []OrderStatus{OrderSellerProcessing, OrderReadyForPickup}, []string{ActorSeller, ActorOperator}},
	ActionExpire:   {OrderCancelled, []OrderStatus{OrderCreated}, []string{ActorSystem}},
}

// Transition returns the status after applying a to a seller-order in status from.
// Illegal moves, including any by a buyer, return a 409 ORDER_NOT_ACTIONABLE error and the unchanged status.
func Transition(from OrderStatus, a Action, actor Actor) (OrderStatus, error) {
	if r, ok := transitions[a]; ok && contains(r.roles, actor.Role) && containsStatus(r.from, from) {
		return r.to, nil
	}
	return from, NewError(ErrConflict, "ORDER_NOT_ACTIONABLE",
		fmt.Sprintf("order cannot be %s from status %s", a, from))
}

// ActionTarget is the status a seller-order has after a; false for an unknown action.
func ActionTarget(a Action) (OrderStatus, bool) {
	r, ok := transitions[a]
	return r.to, ok
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func containsStatus(ss []OrderStatus, s OrderStatus) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

var statusRank = map[OrderStatus]int{
	OrderCreated: 0, OrderSellerProcessing: 1, OrderReadyForPickup: 2, OrderCompleted: 3,
}

// DeriveOrderStatus is the buyer-facing status: the least advanced non-cancelled seller-order
// status, or CANCELLED when all are cancelled. An empty slice is a programmer error and panics.
func DeriveOrderStatus(statuses []OrderStatus) OrderStatus {
	if len(statuses) == 0 {
		panic("domain.DeriveOrderStatus: no seller-order statuses")
	}
	best, found := OrderCancelled, false
	for _, s := range statuses {
		rank, ok := statusRank[s]
		if !ok {
			continue
		}
		if !found || rank < statusRank[best] {
			best, found = s, true
		}
	}
	return best
}

// TabStatuses maps a buyer order-history tab to statuses. ALL returns nil (no filter).
func TabStatuses(tab string) ([]OrderStatus, error) {
	switch tab {
	case "ALL":
		return nil, nil
	case "PROCESSING":
		return []OrderStatus{OrderCreated, OrderSellerProcessing}, nil
	case "READY":
		return []OrderStatus{OrderReadyForPickup}, nil
	case "DONE":
		return []OrderStatus{OrderCompleted, OrderCancelled}, nil
	}
	return nil, NewError(ErrValidationFailed, "VALIDATION_FAILED", "unknown tab: "+tab)
}

// SellerTabStatuses maps a seller order-list tab to statuses.
func SellerTabStatuses(tab string) ([]OrderStatus, error) {
	switch tab {
	case "NEW":
		return []OrderStatus{OrderCreated}, nil
	case "PROCESSING":
		return []OrderStatus{OrderSellerProcessing, OrderReadyForPickup}, nil
	case "DONE":
		return []OrderStatus{OrderCompleted, OrderCancelled}, nil
	}
	return nil, NewError(ErrValidationFailed, "VALIDATION_FAILED", "unknown tab: "+tab)
}

// LineTotal is unitPrice*qty in rupiah, rejecting non-positive quantity, negative price and overflow.
func LineTotal(unitPrice int64, qty int32) (int64, error) {
	if qty <= 0 || unitPrice < 0 {
		return 0, NewError(ErrValidationFailed, "VALIDATION_FAILED", "invalid price or quantity")
	}
	if unitPrice > math.MaxInt64/int64(qty) {
		return 0, NewError(ErrValidationFailed, "VALIDATION_FAILED", "line total overflows")
	}
	return unitPrice * int64(qty), nil
}

// FormatOrderNumber renders NM-YYYYMMDD-NNN using the date in day's own location (pass a WIB time).
func FormatOrderNumber(day time.Time, seq int) string {
	return fmt.Sprintf("NM-%s-%03d", day.Format("20060102"), seq)
}
