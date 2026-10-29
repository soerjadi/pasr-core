package notify_test

import (
	"strings"
	"testing"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/services/notify"
)

func TestMessage_ReadyMentionsOrderAndStore(t *testing.T) {
	title, body := notify.Message(domain.NotifOrderReady, map[string]string{"orderNumber": "NM-20261029-001", "store": "Warung Bu Sari"})
	if title != "Pesanan siap diambil" {
		t.Fatalf("title = %q", title)
	}
	if !strings.Contains(body, "NM-20261029-001") || !strings.Contains(body, "Warung Bu Sari") {
		t.Fatalf("body = %q", body)
	}
}

func TestMessage_EveryKindHasCopy(t *testing.T) {
	for _, k := range []string{domain.NotifNewOrder, domain.NotifOrderAccepted, domain.NotifOrderReady,
		domain.NotifOrderCompleted, domain.NotifOrderRejected, domain.NotifOrderCancelled, domain.NotifStoreVerified} {
		if title, body := notify.Message(k, nil); title == "" || body == "" {
			t.Errorf("%s: empty copy", k)
		}
	}
}
