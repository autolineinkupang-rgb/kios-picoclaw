package kios

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
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
	// Tick setiap 2 menit: notif stok menggating diri sendiri lewat jam/hari,
	// sedangkan notif pesanan baru perlu cek lebih sering.
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	n.tryNotify(ctx)
	n.tryNotifyOrders(ctx)
	n.tryNotifyPendingPileup(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.tryNotify(ctx)
			n.tryNotifyOrders(ctx)
			n.tryNotifyPendingPileup(ctx)
		}
	}
}

// tryNotifyOrders memberi tahu owner saat ada pesanan pending baru dari toko.
// Pada run pertama hanya menetapkan baseline (tidak mengirim riwayat lama).
func (n *NotifService) tryNotifyOrders(ctx context.Context) {
	all, err := n.store.GetAllPesanan(ctx)
	if err != nil || len(all) == 0 {
		return
	}

	maxSeq := 0
	for _, p := range all {
		if s := pesananSeq(p.ID); s > maxSeq {
			maxSeq = s
		}
	}

	v, err := n.store.rdb.Get(ctx, keyNotifPesananLast).Result()
	if err == redis.Nil {
		// Run pertama: jadikan baseline tanpa mengirim notif riwayat.
		_ = n.store.rdb.Set(ctx, keyNotifPesananLast, strconv.Itoa(maxSeq), 0).Err()
		return
	}
	if err != nil {
		return
	}
	last, _ := strconv.Atoi(v)
	if maxSeq <= last {
		return
	}

	var fresh []*Pesanan
	for _, p := range all {
		if p.Status == "pending" && pesananSeq(p.ID) > last {
			fresh = append(fresh, p)
		}
	}
	if len(fresh) > 0 {
		n.sendToOwners(ctx, buildNewOrdersMessage(fresh))
	}
	_ = n.store.rdb.Set(ctx, keyNotifPesananLast, strconv.Itoa(maxSeq), 0).Err()
}

func pesananSeq(id string) int {
	i := strings.LastIndex(id, "-")
	if i < 0 {
		return 0
	}
	n, _ := strconv.Atoi(id[i+1:])
	return n
}

func buildNewOrdersMessage(orders []*Pesanan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🛒 *Pesanan Baru* (%d) — %s WITA\n\n", len(orders), NowWITA().Format("02 Jan 2006 15:04"))
	for _, p := range orders {
		nama := p.NamaPembeli
		if strings.TrimSpace(nama) == "" {
			nama = "Pembeli"
		}
		fmt.Fprintf(&b, "• %s — %s (%d item, %s)\n", p.ID, nama, len(p.Items), FormatRupiah(p.Total))
		if p.Kontak != "" {
			fmt.Fprintf(&b, "  kontak: %s\n", p.Kontak)
		}
	}
	b.WriteString("\nBuka Dashboard → Pesanan untuk memproses ya kak 🙏")
	return b.String()
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
		if p.Stok > p.StokMinimum {
			continue
		}
		label := "menipis"
		if p.Stok == 0 {
			label = "HABIS"
		} else if p.Stok <= p.StokKritis {
			label = "kritis"
		}
		butuh := p.StokMinimum*3 - p.Stok
		if butuh < 0 {
			butuh = 0
		}
		fmt.Fprintf(&b, "- %s [%s]: sisa %d (min %d), perlu restock ±%d\n",
			p.Nama, label, p.Stok, p.StokMinimum, butuh)
		count++
	}
	if count == 0 {
		return "", false
	}

	msg := fmt.Sprintf("⚠️ *Notif Stok* — %s WITA\n\n%s\nSegera restock ya kak! 🙏",
		NowWITA().Format("02 Jan 2006 15:04"), b.String())
	return msg, true
}

// pendingAlertThreshold membaca ambang dari env, default 5.
func pendingAlertThreshold() int {
	if v := strings.TrimSpace(os.Getenv("KIOS_PENDING_ALERT_THRESHOLD")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 5
}

// shouldAlertPileup memutuskan apakah perlu kirim alert dan state berikutnya.
// Alert hanya saat pertama kali mencapai/melewati ambang; reset saat turun.
func shouldAlertPileup(pending, threshold int, state string) (bool, string) {
	if pending >= threshold {
		if state == "alerted" {
			return false, "alerted"
		}
		return true, "alerted"
	}
	if state == "alerted" {
		return false, "clear"
	}
	return false, state
}

// tryNotifyPendingPileup memberi tahu owner saat pesanan pending menumpuk.
func (n *NotifService) tryNotifyPendingPileup(ctx context.Context) {
	all, err := n.store.GetAllPesanan(ctx)
	if err != nil {
		return
	}
	pending := 0
	for _, p := range all {
		if p.Status == "pending" {
			pending++
		}
	}
	state, _ := n.store.rdb.Get(ctx, keyNotifPendingState).Result()
	alert, next := shouldAlertPileup(pending, pendingAlertThreshold(), state)
	if alert {
		n.sendToOwners(ctx, fmt.Sprintf(
			"🛑 *Pesanan Menumpuk* — %s WITA\n\n%d pesanan masih pending. Mohon segera diproses ya kak 🙏",
			NowWITA().Format("02 Jan 2006 15:04"), pending))
	}
	if next != state {
		_ = n.store.rdb.Set(ctx, keyNotifPendingState, next, 0).Err()
	}
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
