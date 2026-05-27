package kios

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// NotifService cek stok menipis secara terjadwal dan kirim notifikasi ke semua owner aktif.
type NotifService struct {
	store   *Store
	msgBus  *bus.MessageBus
	channel string // nama channel, biasanya "telegram"
}

// NewNotifService membuat NotifService baru.
func NewNotifService(store *Store, msgBus *bus.MessageBus, channel string) *NotifService {
	return &NotifService{store: store, msgBus: msgBus, channel: channel}
}

// Start menjalankan loop notifikasi di goroutine terpisah.
// Loop berhenti saat ctx di-cancel.
func (n *NotifService) Start(ctx context.Context) {
	go n.loop(ctx)
}

func (n *NotifService) loop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	// Cek sekali saat startup (saat jam notif sudah lewat tapi belum dikirim hari ini)
	n.tryNotify(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.tryNotify(ctx)
		}
	}
}

// tryNotify cek apakah sekarang waktunya kirim notif, lalu kirim jika ada stok menipis.
func (n *NotifService) tryNotify(ctx context.Context) {
	cfg := n.store.GetConfig(ctx)
	if !cfg.NotifEnabled {
		return
	}

	now := NowWITA()
	jamSekarang := now.Format("15")
	today := now.Format("2006-01-02")

	if jamSekarang != cfg.NotifJam {
		return
	}

	// Cek apakah sudah dikirim hari ini
	lastDate, _ := n.store.rdb.Get(ctx, keyNotifLastDate).Result()
	if lastDate == today {
		return
	}

	msg, ok := n.buildLowStockMessage(ctx)
	if !ok {
		// Tidak ada stok menipis — tandai tetap agar tidak cek terus
		_ = n.store.rdb.Set(ctx, keyNotifLastDate, today, 25*time.Hour).Err()
		return
	}

	n.sendToOwners(ctx, msg)
	_ = n.store.rdb.Set(ctx, keyNotifLastDate, today, 25*time.Hour).Err()
}

// CheckNow kirim notif stok menipis sekarang juga (tanpa cek jadwal/hari).
// Dipanggil manual oleh owner via perintah /notif atau slash command.
func (n *NotifService) CheckNow(ctx context.Context) string {
	msg, ok := n.buildLowStockMessage(ctx)
	if !ok {
		return "Semua stok aman, tidak ada yang menipis kak ✅"
	}
	n.sendToOwners(ctx, msg)
	return "Notifikasi stok menipis sudah dikirim ke semua owner kak 📤"
}

func (n *NotifService) buildLowStockMessage(ctx context.Context) (string, bool) {
	all, err := n.store.GetAllProduk(ctx)
	if err != nil || len(all) == 0 {
		return "", false
	}

	var b strings.Builder
	count := 0
	for _, p := range all {
		if p.Stok <= p.StokMinimum {
			butuh := p.StokMinimum*3 - p.Stok
			if butuh < 0 {
				butuh = 0
			}
			fmt.Fprintf(&b, "- %s: sisa %d (min %d), perlu restock ±%d\n", p.Nama, p.Stok, p.StokMinimum, butuh)
			count++
		}
	}
	if count == 0 {
		return "", false
	}

	msg := fmt.Sprintf("⚠️ *Notif Stok Menipis* — %s WITA\n\n%s\nSegera restock ya kak! 🙏",
		NowWITA().Format("02 Jan 2006 15:04"), b.String())
	return msg, true
}

func (n *NotifService) sendToOwners(ctx context.Context, msg string) {
	users, err := n.store.GetAllUsers(ctx)
	if err != nil {
		logger.WarnCF("kios-notif", "gagal ambil daftar user", map[string]any{"error": err.Error()})
		return
	}

	sent := 0
	for _, u := range users {
		if !u.Aktif || u.Role != "owner" || u.Phone == "" {
			continue
		}
		outCtx := bus.NewOutboundContext(n.channel, u.Phone, "")
		err := n.msgBus.PublishOutbound(ctx, bus.OutboundMessage{
			Channel: n.channel,
			ChatID:  u.Phone,
			Context: outCtx,
			Content: msg,
		})
		if err != nil {
			logger.WarnCF("kios-notif", "gagal kirim notif ke owner", map[string]any{
				"chat_id": u.Phone,
				"error":   err.Error(),
			})
			continue
		}
		sent++
	}
	logger.InfoCF("kios-notif", "notif stok menipis terkirim", map[string]any{"sent": sent})
}
