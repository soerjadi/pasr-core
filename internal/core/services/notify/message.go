package notify

import "github.com/oemah/nusantara/core/internal/core/domain"

// Message returns the Indonesian push copy for a kind. data may carry "orderNumber" and "store".
func Message(kind string, data map[string]string) (title, body string) {
	order, store := data["orderNumber"], data["store"]
	if order == "" {
		order = "kamu"
	}
	if store == "" {
		store = "toko"
	}
	switch kind {
	case domain.NotifNewOrder:
		return "Pesanan baru", "Pesanan " + order + " masuk. Segera cek dan terima pesanannya"
	case domain.NotifOrderAccepted:
		return "Pesanan diterima", "Pesanan " + order + " sedang disiapkan oleh " + store
	case domain.NotifOrderReady:
		return "Pesanan siap diambil", "Pesanan " + order + " sudah bisa diambil di " + store
	case domain.NotifOrderCompleted:
		return "Pesanan selesai", "Pesanan " + order + " dari " + store + " sudah selesai. Terima kasih!"
	case domain.NotifOrderRejected:
		return "Pesanan ditolak", "Maaf, " + store + " tidak dapat memproses pesanan " + order
	case domain.NotifOrderCancelled:
		return "Pesanan dibatalkan", "Pesanan " + order + " dari " + store + " dibatalkan"
	case domain.NotifStoreVerified:
		return "Toko terverifikasi", "Selamat, " + store + " sudah terverifikasi dan tampil di Nusantara"
	}
	return "Notifikasi", "Ada pembaruan untuk kamu"
}
