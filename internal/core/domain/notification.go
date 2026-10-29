package domain

// Notification kinds.
const (
	NotifNewOrder       = "NEW_ORDER"
	NotifOrderAccepted  = "ORDER_ACCEPTED"
	NotifOrderReady     = "ORDER_READY"
	NotifOrderCompleted = "ORDER_COMPLETED"
	NotifOrderRejected  = "ORDER_REJECTED"
	NotifOrderCancelled = "ORDER_CANCELLED"
	NotifStoreVerified  = "STORE_VERIFIED"
)

// ValidNotificationKind reports whether k is a known kind.
func ValidNotificationKind(k string) bool {
	switch k {
	case NotifNewOrder, NotifOrderAccepted, NotifOrderReady, NotifOrderCompleted,
		NotifOrderRejected, NotifOrderCancelled, NotifStoreVerified:
		return true
	}
	return false
}

// Notification is a push message queued for one user. Data carries string values only (FCM data payload),
// for example "orderNumber", "orderId" and "store".
type Notification struct {
	ID     int64             `json:"id"`
	UserID int64             `json:"userId"`
	Kind   string            `json:"kind"`
	Title  string            `json:"title"`
	Body   string            `json:"body"`
	Data   map[string]string `json:"data"`
}
