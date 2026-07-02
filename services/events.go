package services

import (
	"encoding/json"
	"sync"
	"time"
)

// Hub SSE sederhana untuk update realtime ke browser admin: device sync →
// Broadcast → semua dashboard yang terhubung menerima event dan me-refresh
// datanya sendiri (payload event sengaja minim, hanya "apa berubah di mana").
type sseHub struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

var EventsHub = &sseHub{subs: map[chan string]struct{}{}}

// Subscribe mendaftarkan satu koneksi browser. Channel ber-buffer supaya
// broadcast tidak pernah memblokir jalur sync; event yang meluap di-drop
// (klien toh me-refresh penuh saat menerima event apa pun).
func (h *sseHub) Subscribe() chan string {
	ch := make(chan string, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *sseHub) Unsubscribe(ch chan string) {
	h.mu.Lock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// Broadcast mengirim event {type, outlet_id, at} ke semua koneksi. Non-blocking.
func (h *sseHub) Broadcast(eventType, outletID string) {
	payload, _ := json.Marshal(map[string]string{
		"type":      eventType,
		"outlet_id": outletID,
		"at":        time.Now().UTC().Format(time.RFC3339),
	})
	msg := string(payload)
	h.mu.Lock()
	for ch := range h.subs {
		select {
		case ch <- msg:
		default: // subscriber lambat/penuh → drop, jangan blokir sync
		}
	}
	h.mu.Unlock()
}

// BroadcastSync dipanggil dari jalur simpan data outlet (transaksi, order, shift,
// kas, void item). eventType = jenis entitas yang berubah.
func BroadcastSync(eventType, outletID string) {
	EventsHub.Broadcast(eventType, outletID)
}
