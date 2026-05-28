# Tugas 1 — Laporan & Alert Bot (Implementation Plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mengaktifkan & menyempurnakan laporan/alert bot ke Telegram: laporan harian kaya dari `/api/summary` (dengan fallback), plus alert stok kritis/habis dan pesanan pending menumpuk.

**Architecture:** Bot Go sudah membaca Redis Upstash bersama secara langsung dan memanggil `/api/summary` (HMAC) untuk ringkasan terhitung. Plan ini memakai ulang `fetchDashboardSummary`/`formatDashboardSummary` yang sudah ada, dan menambah klasifikasi alert di `NotifService`. Login-kode tidak dipakai.

**Tech Stack:** Go 1.25, `github.com/redis/go-redis/v9`, miniredis untuk test, cron service internal (`pkg/cron`).

---

## Struktur berkas

- `pkg/tools/kios/report.go` — modifikasi `DailyReportText`, tambah var `summaryFetcher` (injectable).
- `pkg/tools/kios/report_test.go` — **baru**, test laporan harian (sukses + fallback).
- `pkg/tools/kios/notif.go` — modifikasi `buildLowStockMessage` (klasifikasi kritis/habis), tambah `tryNotifyPendingPileup` + helper murni `shouldAlertPileup`, tambah `pendingAlertThreshold`.
- `pkg/tools/kios/notif_test.go` — **baru**, test klasifikasi stok + helper pileup.
- `pkg/tools/kios/store.go` — tambah konstanta `keyNotifPendingState`.
- `.env.example` / `DEPLOY-RAILWAY.md` — dokumentasi env (Task 4).

Catatan referensi:
- `DashboardSummary`, `fetchDashboardSummary`, `formatDashboardSummary`, `serviceAuthHeader` ada di `pkg/tools/kios/dashboard_summary.go`.
- Helper test `newTestStore(t)` di `kios_test.go:970` (miniredis + `NewStoreWithClient`).
- `Produk` punya `Stok`, `StokMinimum`, `StokKritis` (`store.go:21-31`). `SetProduk` di `store.go:303`. `GetAllPesanan` dipakai di `notif.go`. Konstanta notif di `store.go:153-156`.

---

## Task 1: Laporan harian kaya via /api/summary (dengan fallback)

**Files:**
- Create: `pkg/tools/kios/report_test.go`
- Modify: `pkg/tools/kios/report.go`

- [ ] **Step 1: Tulis test gagal**

```go
package kios

import (
	"context"
	"strings"
	"testing"
)

func TestDailyReportText_UsesSummaryWhenAvailable(t *testing.T) {
	s := newTestStore(t)
	orig := summaryFetcher
	t.Cleanup(func() { summaryFetcher = orig })
	summaryFetcher = func(ctx context.Context) (*DashboardSummary, error) {
		sum := &DashboardSummary{OK: true, Waktu: "2026-05-28 18:00"}
		sum.PenjualanHariIni.Omzet = 250000
		sum.PenjualanHariIni.Transaksi = 12
		return sum, nil
	}
	got := DailyReportText(context.Background(), s)
	if !strings.Contains(got, "Laporan Harian") || !strings.Contains(got, "Omzet") {
		t.Errorf("expected rich summary with Omzet, got: %s", got)
	}
}

func TestDailyReportText_FallbackOnError(t *testing.T) {
	s := newTestStore(t)
	orig := summaryFetcher
	t.Cleanup(func() { summaryFetcher = orig })
	summaryFetcher = func(ctx context.Context) (*DashboardSummary, error) {
		return nil, context.DeadlineExceeded
	}
	got := DailyReportText(context.Background(), s)
	if !strings.Contains(got, "Laporan Harian") {
		t.Errorf("expected fallback report header, got: %s", got)
	}
}
```

- [ ] **Step 2: Jalankan test — harus GAGAL**

Run: `go test ./pkg/tools/kios/ -run TestDailyReportText -v`
Expected: GAGAL kompilasi — `undefined: summaryFetcher`.

- [ ] **Step 3: Modifikasi `report.go`**

Ganti fungsi `DailyReportText` (sekarang `report.go:16-23`) dan tambahkan var injectable. Pastikan import `os` dan `strings` tetap ada (sudah ada di file).

```go
// summaryFetcher dipisah agar bisa di-stub di test.
var summaryFetcher = fetchDashboardSummary

// DailyReportText menyusun teks laporan harian. Memakai ringkasan kaya dari
// /api/summary bila tersedia; bila gagal (dashboard mati / env belum diset),
// jatuh ke ringkasan Go dari Redis langsung agar laporan tetap terkirim.
func DailyReportText(ctx context.Context, store *Store) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("KIOS_DASHBOARD_URL")), "/")
	if s, err := summaryFetcher(ctx); err == nil && s != nil && s.OK {
		return "📅 Laporan Harian Otomatis\n" + formatDashboardSummary(s, base, 0)
	}
	res := NewLaporanTool(store).Execute(ctx, map[string]any{"action": "ringkas"})
	text := res.ForLLM
	if strings.TrimSpace(text) == "" {
		return "📅 Laporan harian: belum ada data hari ini."
	}
	return "📅 Laporan Harian Otomatis\n" + text
}
```

- [ ] **Step 4: Jalankan test — harus LULUS**

Run: `go test ./pkg/tools/kios/ -run TestDailyReportText -v`
Expected: PASS (kedua test).

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/kios/report.go pkg/tools/kios/report_test.go
git commit -m "feat(kios): laporan harian bot pakai /api/summary dengan fallback"
```

---

## Task 2: Alert stok kritis/habis

**Files:**
- Create: `pkg/tools/kios/notif_test.go`
- Modify: `pkg/tools/kios/notif.go` (`buildLowStockMessage`, `notif.go:163-188`)

- [ ] **Step 1: Tulis test gagal**

```go
package kios

import (
	"context"
	"strings"
	"testing"
)

func TestBuildLowStockMessage_ClassifiesCriticalAndOut(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.SetProduk(ctx, &Produk{ID: "P1", Nama: "Beras", Stok: 0, StokMinimum: 5, StokKritis: 2})
	_ = s.SetProduk(ctx, &Produk{ID: "P2", Nama: "Gula", Stok: 2, StokMinimum: 5, StokKritis: 2})
	_ = s.SetProduk(ctx, &Produk{ID: "P3", Nama: "Kopi", Stok: 50, StokMinimum: 5, StokKritis: 2})

	n := NewNotifService(s, nil, "telegram")
	msg, ok := n.buildLowStockMessage(ctx)
	if !ok {
		t.Fatal("expected a message")
	}
	if !strings.Contains(msg, "Beras [HABIS]") {
		t.Errorf("expected HABIS label, got: %s", msg)
	}
	if !strings.Contains(msg, "Gula [kritis]") {
		t.Errorf("expected kritis label, got: %s", msg)
	}
	if strings.Contains(msg, "Kopi") {
		t.Errorf("Kopi (aman) should not appear, got: %s", msg)
	}
}
```

- [ ] **Step 2: Jalankan test — harus GAGAL**

Run: `go test ./pkg/tools/kios/ -run TestBuildLowStockMessage -v`
Expected: GAGAL — label `[HABIS]`/`[kritis]` belum ada di output.

- [ ] **Step 3: Modifikasi `buildLowStockMessage` di `notif.go`**

```go
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
```

- [ ] **Step 4: Jalankan test — harus LULUS**

Run: `go test ./pkg/tools/kios/ -run TestBuildLowStockMessage -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/tools/kios/notif.go pkg/tools/kios/notif_test.go
git commit -m "feat(kios): alert stok klasifikasi kritis/habis"
```

---

## Task 3: Alert pesanan pending menumpuk

**Files:**
- Modify: `pkg/tools/kios/store.go` (tambah konstanta key, dekat `store.go:156`)
- Modify: `pkg/tools/kios/notif.go` (tambah helper murni + `tryNotifyPendingPileup`, panggil di `loop`)
- Modify: `pkg/tools/kios/notif_test.go` (tambah test helper murni)

- [ ] **Step 1: Tambah konstanta key di `store.go`**

Setelah baris `keyNotifPesananLast = ...` (`store.go:156`), tambahkan:

```go
	keyNotifPendingState = "kios:notif:pending_state" // "alerted" | "clear"
```

- [ ] **Step 2: Tulis test gagal (helper murni)**

Tambahkan ke `notif_test.go`:

```go
func TestShouldAlertPileup(t *testing.T) {
	cases := []struct {
		pending, threshold int
		state              string
		wantAlert          bool
		wantState          string
	}{
		{5, 5, "clear", true, "alerted"},   // mencapai ambang → alert
		{6, 5, "alerted", false, "alerted"}, // masih menumpuk → tidak spam
		{2, 5, "alerted", false, "clear"},   // turun di bawah ambang → reset
		{1, 5, "clear", false, "clear"},     // aman → diam
	}
	for _, c := range cases {
		gotAlert, gotState := shouldAlertPileup(c.pending, c.threshold, c.state)
		if gotAlert != c.wantAlert || gotState != c.wantState {
			t.Errorf("shouldAlertPileup(%d,%d,%q)=(%v,%q) want (%v,%q)",
				c.pending, c.threshold, c.state, gotAlert, gotState, c.wantAlert, c.wantState)
		}
	}
}
```

- [ ] **Step 3: Jalankan test — harus GAGAL**

Run: `go test ./pkg/tools/kios/ -run TestShouldAlertPileup -v`
Expected: GAGAL — `undefined: shouldAlertPileup`.

- [ ] **Step 4: Implementasi di `notif.go`**

Tambahkan import `os` ke blok import `notif.go` (`strconv` sudah ada). Lalu tambahkan:

```go
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
```

Lalu panggil di `loop()` (`notif.go:33-51`) bersama `tryNotifyOrders` — di run pertama dan di setiap tick:

```go
	n.tryNotify(ctx)
	n.tryNotifyOrders(ctx)
	n.tryNotifyPendingPileup(ctx)
```

(dan tambahkan baris `n.tryNotifyPendingPileup(ctx)` di dalam `case <-ticker.C:`).

- [ ] **Step 5: Jalankan test — harus LULUS, lalu seluruh paket**

Run: `go test ./pkg/tools/kios/ -run TestShouldAlertPileup -v`
Expected: PASS.
Run: `go test ./pkg/tools/kios/`
Expected: ok (semua test paket lulus).

- [ ] **Step 6: Commit**

```bash
git add pkg/tools/kios/store.go pkg/tools/kios/notif.go pkg/tools/kios/notif_test.go
git commit -m "feat(kios): alert pesanan pending menumpuk dengan ambang konfigurable"
```

---

## Task 4: Dokumentasi env aktivasi

**Files:**
- Modify: `.env.example`
- Modify: `DEPLOY-RAILWAY.md`

- [ ] **Step 1: Tambah entri env**

Tambahkan ke `.env.example` (dan jelaskan di `DEPLOY-RAILWAY.md`) variabel berikut, dengan komentar singkat Bahasa Indonesia:

```
# Laporan harian otomatis ke Telegram (aktif bila diisi)
KIOS_REPORT_CHAT=
# Opsional: override jadwal cron (default 0 18 * * *, TZ Asia/Makassar)
KIOS_REPORT_CRON=
# Base URL dashboard untuk /api/summary
KIOS_DASHBOARD_URL=
# Rahasia HMAC service — HARUS sama dengan KIOS_SERVICE_SECRET di dashboard
KIOS_SERVICE_SECRET=
# Ambang alert pesanan pending menumpuk (default 5)
KIOS_PENDING_ALERT_THRESHOLD=
```

- [ ] **Step 2: Verifikasi build & test**

Run: `go build ./... && go test ./pkg/tools/kios/`
Expected: sukses build + test.

- [ ] **Step 3: Commit**

```bash
git add .env.example DEPLOY-RAILWAY.md
git commit -m "docs(kios): env laporan & alert bot (report chat, dashboard url, ambang pending)"
```

---

## Self-review (sudah dijalankan penulis)

- **Cakupan spec:** laporan terjadwal (cron sudah ada + Task 1 perkaya), on-demand (tool `kios_laporan` sudah ada, tak berubah), alert stok kritis/habis (Task 2), pesanan menumpuk (Task 3), aktivasi env (Task 4). ✓
- **Placeholder:** tidak ada — semua langkah berisi kode/perintah nyata.
- **Konsistensi tipe:** `summaryFetcher` (Task 1) dipakai konsisten; `shouldAlertPileup`/`pendingAlertThreshold`/`keyNotifPendingState` (Task 3) konsisten antar langkah; memakai `SetProduk`, `GetAllPesanan`, `Produk.StokKritis` yang terverifikasi ada.

## Verifikasi manual akhir

- Set `KIOS_REPORT_CHAT` + `KIOS_DASHBOARD_URL` + `KIOS_SERVICE_SECRET` lokal, jalankan `./picoclaw gateway`, picu laporan (atau tunggu cron) → cek pesan masuk Telegram berisi omzet/laba/terlaris.
- Matikan dashboard → laporan tetap terkirim (fallback Go).
- Buat ≥ ambang pesanan pending → cek alert menumpuk; proses sebagian → tidak spam; pastikan reset saat turun.
