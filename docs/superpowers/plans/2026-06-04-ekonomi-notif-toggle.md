# Ekonomi Mode + Toggle Notifikasi + Kategori Panduan — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tambah toggle on/off notif stok & pesanan via `/notif` Telegram, env var `KIOS_EKONOMI_MODE`/`KIOS_JAM_AKTIF` untuk matikan laporan+backup, dan refactor `panduanText` dengan pemisah kategori.

**Architecture:** `KiosConfig` diperluas dengan `NotifPesananEnabled`; helper `service_gate.go` (fungsi murni, testable) membaca env var; `/notif` command diperluas dengan sub-args; `panduanText` direfactor dengan header kategori. Tidak ada perubahan ke picoclaw upstream.

**Tech Stack:** Go 1.22 (tags `goolm,stdjson`), Redis (miniredis untuk test), Telegram slash commands (0-token).

**Prasyarat toolchain:**
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
```

Perintah test kanonik:
```bash
go test -tags goolm,stdjson ./pkg/tools/kios/...
```

---

## File Structure

| File | Tanggung Jawab | Aksi |
|---|---|---|
| `pkg/tools/kios/store.go` | Tambah `NotifPesananEnabled` ke `KiosConfig` | Modify |
| `pkg/tools/kios/store_access.go` | Update `defaultConfig()` — default `NotifPesananEnabled: true` | Modify |
| `pkg/tools/kios/service_gate.go` | **BARU** — `IsEkonomiMode()`, `IsJamAktif()`, `IsServiceAktif()` | Create |
| `pkg/tools/kios/service_gate_test.go` | **BARU** — test jam aktif (normal, overnight, kosong) | Create |
| `pkg/tools/kios/notif.go` | Guard `NotifPesananEnabled` di `tryNotifyOrders` + `tryNotifyPendingPileup` | Modify |
| `pkg/tools/kios/commands.go` | Extend `/notif` dengan sub-args on/off; refactor `panduanText` | Modify |
| `pkg/tools/kios/report.go` | Guard `IsServiceAktif()` di `EnsureDailyReportJob` | Modify |
| `pkg/tools/kios/backup.go` | Guard `IsServiceAktif()` di `EnsureDailyBackupJob` | Modify |
| `kios-dashboard/src/lib/types.ts` | Tambah `notif_pesanan_enabled?: boolean` ke `KiosConfig` | Modify |
| `deploy/env.example` | Dokumentasikan dua env var baru | Modify |

---

## Task 1: `NotifPesananEnabled` ke `KiosConfig` + default

**Files:**
- Modify: `pkg/tools/kios/store.go` (struct `KiosConfig`)
- Modify: `pkg/tools/kios/store_access.go` (fungsi `defaultConfig`)

### Step 1: Tambah field ke `KiosConfig` di `store.go`

- [ ] Di `pkg/tools/kios/store.go`, dalam struct `KiosConfig`, tambahkan field baru setelah `NotifJam`:

```go
// NotifPesananEnabled mengaktifkan notifikasi pesanan baru & menumpuk. Default true.
NotifPesananEnabled bool `json:"notif_pesanan_enabled"`
```

### Step 2: Update `defaultConfig` di `store_access.go`

- [ ] Di `pkg/tools/kios/store_access.go`, baris 23, ubah `defaultConfig()`:

Dari:
```go
return KiosConfig{AutoLearnEnabled: true, NotifEnabled: true, NotifJam: "07"}
```

Menjadi:
```go
return KiosConfig{AutoLearnEnabled: true, NotifEnabled: true, NotifJam: "07", NotifPesananEnabled: true}
```

### Step 3: Build check

- [ ] Jalankan build untuk pastikan kompilasi bersih:

```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
cd /home/kevinman/Publik/project/kios-picoclaw
go build -tags goolm,stdjson ./pkg/tools/kios/...
```

Expected: no output (bersih).

### Step 4: Update TS mirror

- [ ] Di `kios-dashboard/src/lib/types.ts`, dalam `interface KiosConfig`, tambahkan setelah `notif_jam`:

```typescript
notif_pesanan_enabled?: boolean; // default true
```

### Step 5: Commit Task 1

- [ ]
```bash
git add pkg/tools/kios/store.go pkg/tools/kios/store_access.go kios-dashboard/src/lib/types.ts
git commit -m "feat(kios): tambah NotifPesananEnabled ke KiosConfig (default true)"
```

---

## Task 2: `service_gate.go` — helper env var ekonomi + jam aktif

**Files:**
- Create: `pkg/tools/kios/service_gate.go`
- Create: `pkg/tools/kios/service_gate_test.go`

### Step 1: Tulis test yang gagal

- [ ] Buat file `pkg/tools/kios/service_gate_test.go`:

```go
package kios

import (
	"os"
	"testing"
)

func TestIsJamAktifKosong(t *testing.T) {
	os.Unsetenv("KIOS_JAM_AKTIF")
	// Env kosong → selalu aktif
	for hour := 0; hour < 24; hour++ {
		if !isJamAktifAt(hour) {
			t.Errorf("isJamAktifAt(%d) = false, want true (env kosong)", hour)
		}
	}
}

func TestIsJamAktifNormal(t *testing.T) {
	os.Setenv("KIOS_JAM_AKTIF", "07-22")
	defer os.Unsetenv("KIOS_JAM_AKTIF")

	aktif := []int{7, 10, 15, 21}
	for _, h := range aktif {
		if !isJamAktifAt(h) {
			t.Errorf("isJamAktifAt(%d) = false, want true (07-22)", h)
		}
	}
	nonAktif := []int{0, 3, 6, 22, 23}
	for _, h := range nonAktif {
		if isJamAktifAt(h) {
			t.Errorf("isJamAktifAt(%d) = true, want false (07-22)", h)
		}
	}
}

func TestIsJamAktifOvernight(t *testing.T) {
	os.Setenv("KIOS_JAM_AKTIF", "22-06")
	defer os.Unsetenv("KIOS_JAM_AKTIF")

	aktif := []int{22, 23, 0, 3, 5}
	for _, h := range aktif {
		if !isJamAktifAt(h) {
			t.Errorf("isJamAktifAt(%d) = false, want true (22-06)", h)
		}
	}
	nonAktif := []int{6, 10, 15, 21}
	for _, h := range nonAktif {
		if isJamAktifAt(h) {
			t.Errorf("isJamAktifAt(%d) = true, want false (22-06)", h)
		}
	}
}

func TestIsEkonomiMode(t *testing.T) {
	os.Unsetenv("KIOS_EKONOMI_MODE")
	if IsEkonomiMode() {
		t.Error("IsEkonomiMode() = true, want false (env kosong)")
	}

	os.Setenv("KIOS_EKONOMI_MODE", "true")
	defer os.Unsetenv("KIOS_EKONOMI_MODE")
	if !IsEkonomiMode() {
		t.Error("IsEkonomiMode() = false, want true")
	}

	// Nilai selain "true" dianggap false
	os.Setenv("KIOS_EKONOMI_MODE", "1")
	if IsEkonomiMode() {
		t.Error("IsEkonomiMode() = true for '1', want false (hanya 'true' yang valid)")
	}
}

func TestIsServiceAktif(t *testing.T) {
	os.Unsetenv("KIOS_EKONOMI_MODE")
	os.Unsetenv("KIOS_JAM_AKTIF")
	if !IsServiceAktif() {
		t.Error("IsServiceAktif() = false tanpa env var, want true")
	}

	os.Setenv("KIOS_EKONOMI_MODE", "true")
	defer os.Unsetenv("KIOS_EKONOMI_MODE")
	if IsServiceAktif() {
		t.Error("IsServiceAktif() = true saat KIOS_EKONOMI_MODE=true, want false")
	}
}
```

- [ ] Jalankan test untuk konfirmasi gagal (kompilasi):

```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
cd /home/kevinman/Publik/project/kios-picoclaw
go test -tags goolm,stdjson ./pkg/tools/kios/... -run "TestIsJamAktif|TestIsEkonomiMode|TestIsServiceAktif" -v 2>&1 | tail -10
```

Expected: FAIL — `IsEkonomiMode`, `IsServiceAktif`, `isJamAktifAt` undefined.

### Step 2: Buat `service_gate.go`

- [ ] Buat file `pkg/tools/kios/service_gate.go`:

```go
package kios

import (
	"os"
	"strconv"
	"strings"
)

// IsEkonomiMode returns true jika KIOS_EKONOMI_MODE=true (exact match, lowercase).
// Nilai lain (mis. "1", "yes") dianggap false agar perubahan eksplisit.
func IsEkonomiMode() bool {
	return strings.TrimSpace(os.Getenv("KIOS_EKONOMI_MODE")) == "true"
}

// IsJamAktif memeriksa apakah jam WITA saat ini masuk dalam window KIOS_JAM_AKTIF.
// Format: "HH-HH" (mis. "07-22" atau overnight "22-06").
// Jika env kosong atau format tidak valid → selalu true.
func IsJamAktif() bool {
	return isJamAktifAt(NowWITA().Hour())
}

// isJamAktifAt adalah versi testable dari IsJamAktif yang menerima jam eksplisit.
func isJamAktifAt(hour int) bool {
	val := strings.TrimSpace(os.Getenv("KIOS_JAM_AKTIF"))
	if val == "" {
		return true
	}
	parts := strings.SplitN(val, "-", 2)
	if len(parts) != 2 {
		return true
	}
	from, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	to, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || from < 0 || from > 23 || to < 0 || to > 23 {
		return true
	}
	if from <= to {
		// Normal range: 07-22 → aktif jam 7,8,...,21
		return hour >= from && hour < to
	}
	// Overnight range: 22-06 → aktif jam 22,23,0,1,...,5
	return hour >= from || hour < to
}

// IsServiceAktif returns false jika ekonomi mode aktif.
// Dipakai oleh EnsureDailyReportJob dan EnsureDailyBackupJob.
func IsServiceAktif() bool {
	if IsEkonomiMode() {
		return false
	}
	return IsJamAktif()
}
```

### Step 3: Jalankan test dan pastikan lulus

- [ ]
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
cd /home/kevinman/Publik/project/kios-picoclaw
go test -tags goolm,stdjson ./pkg/tools/kios/... -run "TestIsJamAktif|TestIsEkonomiMode|TestIsServiceAktif" -v 2>&1 | tail -15
```

Expected: semua PASS.

### Step 4: Jalankan semua test

- [ ]
```bash
go test -tags goolm,stdjson ./pkg/tools/kios/... 2>&1 | tail -5
```

Expected: `ok github.com/sipeed/picoclaw/pkg/tools/kios`.

### Step 5: Commit Task 2

- [ ]
```bash
git add pkg/tools/kios/service_gate.go pkg/tools/kios/service_gate_test.go
git commit -m "feat(kios): service_gate — IsEkonomiMode + IsJamAktif + IsServiceAktif"
```

---

## Task 3: Guard di `notif.go` + guard di `report.go` + `backup.go`

**Files:**
- Modify: `pkg/tools/kios/notif.go`
- Modify: `pkg/tools/kios/report.go`
- Modify: `pkg/tools/kios/backup.go`

### Step 1: Guard `NotifPesananEnabled` di `notif.go`

- [ ] Di `pkg/tools/kios/notif.go`, fungsi `tryNotifyOrders`, tambahkan guard di AWAL fungsi (sebelum `all, err := n.store.GetAllPesanan(ctx)`):

```go
func (n *NotifService) tryNotifyOrders(ctx context.Context) {
	cfg := n.store.GetConfig(ctx)
	if !cfg.NotifPesananEnabled {
		return
	}
	// ... kode yang sudah ada ...
```

- [ ] Di fungsi `tryNotifyPendingPileup`, tambahkan guard yang sama di awal:

```go
func (n *NotifService) tryNotifyPendingPileup(ctx context.Context) {
	cfg := n.store.GetConfig(ctx)
	if !cfg.NotifPesananEnabled {
		return
	}
	// ... kode yang sudah ada ...
```

### Step 2: Guard `IsServiceAktif` di `report.go`

- [ ] Di `pkg/tools/kios/report.go`, fungsi `EnsureDailyReportJob`, tambahkan guard setelah baris `chat := ...`:

Dari:
```go
func EnsureDailyReportJob(cs *cron.CronService) error {
	chat := strings.TrimSpace(os.Getenv("KIOS_REPORT_CHAT"))
	if chat == "" {
		return nil
	}
```

Menjadi:
```go
func EnsureDailyReportJob(cs *cron.CronService) error {
	chat := strings.TrimSpace(os.Getenv("KIOS_REPORT_CHAT"))
	if chat == "" {
		return nil
	}
	if IsEkonomiMode() {
		return nil // KIOS_EKONOMI_MODE=true: laporan harian dinonaktifkan
	}
```

### Step 3: Guard `IsEkonomiMode` di `backup.go`

- [ ] Di `pkg/tools/kios/backup.go`, fungsi `EnsureDailyBackupJob`, tambahkan guard setelah baris `chat := ...`. Cari baris:

```go
func EnsureDailyBackupJob(cs *cron.CronService) error {
	chat := strings.TrimSpace(os.Getenv("KIOS_BACKUP_CHAT"))
	if chat == "" {
		return nil
	}
```

Tambahkan setelah `if chat == ""`:
```go
	if IsEkonomiMode() {
		return nil // KIOS_EKONOMI_MODE=true: backup harian dinonaktifkan
	}
```

### Step 4: Build + semua test

- [ ]
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
cd /home/kevinman/Publik/project/kios-picoclaw
go test -tags goolm,stdjson ./pkg/tools/kios/... 2>&1 | tail -5
```

Expected: `ok github.com/sipeed/picoclaw/pkg/tools/kios`.

### Step 5: Commit Task 3

- [ ]
```bash
git add pkg/tools/kios/notif.go pkg/tools/kios/report.go pkg/tools/kios/backup.go
git commit -m "feat(kios): guard NotifPesananEnabled di notif + IsEkonomiMode di report+backup"
```

---

## Task 4: Extend `/notif` command dengan sub-args on/off

**Files:**
- Modify: `pkg/tools/kios/commands.go` (bagian command `/notif`)

### Step 1: Tulis test

- [ ] Di `pkg/tools/kios/kios_test.go` (atau buat file `notif_cmd_test.go` jika lebih bersih), tambahkan:

```go
func TestNotifCommandStatus(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	// Default: kedua notif aktif
	cfg := s.GetConfig(ctx)
	if !cfg.NotifEnabled {
		t.Error("NotifEnabled default harus true")
	}
	if !cfg.NotifPesananEnabled {
		t.Error("NotifPesananEnabled default harus true")
	}
}

func TestNotifCommandTogglePesanan(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Matikan notif pesanan
	cfg := s.GetConfig(ctx)
	cfg.NotifPesananEnabled = false
	if err := s.SaveConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	// Verifikasi tersimpan
	loaded := s.GetConfig(ctx)
	if loaded.NotifPesananEnabled {
		t.Error("NotifPesananEnabled harus false setelah disimpan")
	}

	// Aktifkan kembali
	loaded.NotifPesananEnabled = true
	_ = s.SaveConfig(ctx, loaded)
	if !s.GetConfig(ctx).NotifPesananEnabled {
		t.Error("NotifPesananEnabled harus true setelah diaktifkan kembali")
	}
}
```

- [ ] Jalankan test:
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
go test -tags goolm,stdjson ./pkg/tools/kios/... -run "TestNotifCommand" -v 2>&1 | tail -10
```

Expected: PASS (tidak ada kode baru yang dibutuhkan, hanya verifikasi struct).

### Step 2: Ganti handler `/notif` di `commands.go`

- [ ] Di `pkg/tools/kios/commands.go`, cari blok command `"notif"` (sekitar baris 371) dan ganti seluruhnya dengan:

```go
{
    Name:        "notif",
    Description: "Toggle notifikasi stok/pesanan atau kirim notif sekarang (owner)",
    Usage:       "/notif [stok on|off] [pesanan on|off]",
    Handler: func(ctx context.Context, req commands.Request, _ *commands.Runtime) error {
        ctx = withSender(ctx, req)
        role, _, refusal := resolveRole(ctx, store)
        if refusal != nil {
            return reply(req, refusal.ForLLM)
        }
        if r := requireOwner(role); r != nil {
            return reply(req, r.ForLLM)
        }

        arg := strings.ToLower(strings.TrimSpace(argAfter(req.Text)))

        // /notif stok on|off
        if strings.HasPrefix(arg, "stok ") {
            state := strings.TrimPrefix(arg, "stok ")
            cfg := store.GetConfig(ctx)
            switch state {
            case "on":
                cfg.NotifEnabled = true
                _ = store.SaveConfig(ctx, cfg)
                return reply(req, "✅ Notifikasi stok menipis *diaktifkan* kak.")
            case "off":
                cfg.NotifEnabled = false
                _ = store.SaveConfig(ctx, cfg)
                return reply(req, "🔕 Notifikasi stok menipis *dimatikan* kak.")
            default:
                return reply(req, "Ketik `/notif stok on` atau `/notif stok off` ya kak.")
            }
        }

        // /notif pesanan on|off
        if strings.HasPrefix(arg, "pesanan ") {
            state := strings.TrimPrefix(arg, "pesanan ")
            cfg := store.GetConfig(ctx)
            switch state {
            case "on":
                cfg.NotifPesananEnabled = true
                _ = store.SaveConfig(ctx, cfg)
                return reply(req, "✅ Notifikasi pesanan baru *diaktifkan* kak.")
            case "off":
                cfg.NotifPesananEnabled = false
                _ = store.SaveConfig(ctx, cfg)
                return reply(req, "🔕 Notifikasi pesanan baru *dimatikan* kak.")
            default:
                return reply(req, "Ketik `/notif pesanan on` atau `/notif pesanan off` ya kak.")
            }
        }

        // /notif tanpa arg → tampilkan status + kirim notif sekarang
        cfg := store.GetConfig(ctx)
        stokStatus := "✅ Aktif"
        if !cfg.NotifEnabled {
            stokStatus = "❌ Nonaktif"
        }
        pesananStatus := "✅ Aktif"
        if !cfg.NotifPesananEnabled {
            pesananStatus = "❌ Nonaktif"
        }
        status := fmt.Sprintf(
            "🔔 *Status Notifikasi*\n\nStok menipis  : %s\nPesanan baru  : %s\n\n"+
                "Ubah dengan:\n• /notif stok on|off\n• /notif pesanan on|off",
            stokStatus, pesananStatus,
        )
        if notifSvc != nil && arg == "" {
            // Kirim notif stok sekarang jika tidak ada arg
            nowResult := notifSvc.CheckNow(ctx)
            return reply(req, status+"\n\n"+nowResult)
        }
        return reply(req, status)
    },
},
```

### Step 3: Jalankan semua test

- [ ]
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
go test -tags goolm,stdjson ./pkg/tools/kios/... 2>&1 | tail -5
```

Expected: `ok github.com/sipeed/picoclaw/pkg/tools/kios`.

### Step 4: Commit Task 4

- [ ]
```bash
git add pkg/tools/kios/commands.go pkg/tools/kios/kios_test.go
git commit -m "feat(kios): extend /notif — toggle stok+pesanan on/off + status display"
```

---

## Task 5: Refactor `panduanText` dengan pemisah kategori

**Files:**
- Modify: `pkg/tools/kios/commands.go` (konstanta `panduanText`)

### Step 1: Ganti `panduanText`

- [ ] Di `pkg/tools/kios/commands.go`, ganti seluruh konstanta `panduanText` (baris 19–58) dengan:

```go
const panduanText = `📖 *Panduan Kios Cerdas*

━━━ 🛒 KASIR & TRANSAKSI ━━━
/stok [nama]        — cek stok / cari produk
/jual <produk> <jml> — catat penjualan + struk
/jualmassal <p> <jml>, ... — jual banyak sekaligus
/laporan            — ringkasan penjualan hari ini
/batal <TRX-id>     — batalkan transaksi (owner)
/shift              — buka/cek/tutup shift kasir

━━━ 💰 HARGA & INFO ━━━
/harga <produk>     — lihat harga jual & modal
/promo              — daftar promo & diskon aktif
/pasar [produk]     — bandingkan harga vs pasar
/menipis            — produk yang hampir habis
/produk [nama]      — daftar / detail produk
/suplier [nama]     — info supplier & banding harga
/qris               — tampilkan QRIS kios

━━━ 📦 STOK KHUSUS ━━━
/pulsa [nominal]    — cek saldo / jual pulsa
/bensin [nama liter] — cek stok / jual bensin

━━━ 🔔 NOTIFIKASI (Owner) ━━━
/notif              — status notifikasi + cek sekarang
/notif stok on|off  — aktifkan/matikan notif stok menipis
/notif pesanan on|off — aktifkan/matikan notif pesanan baru

━━━ ⚙️ OWNER ━━━
/backup             — export semua data ke JSON
/template           — download template Excel
/isipulsa <jumlah>  — top-up saldo modal pulsa
/isibensin <nama> <liter> <harga> — restock bensin

━━━ 💬 TANYA AI (pakai token) ━━━
• "laporan minggu ini"
• "tambah produk baru gula 1 kg harga 14.000"
• "restock minyak 24 botol harga beli 12.000"
• "promo diskon 10% untuk susu bulan ini"

💡 *Tips:* Perintah /xxx tidak pakai token AI — gratis!
Sebut nominal bayar saat jual agar dapat kembalian.`
```

### Step 2: Build check

- [ ]
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
go build -tags goolm,stdjson ./pkg/tools/kios/...
```

Expected: no output.

### Step 3: Commit Task 5

- [ ]
```bash
git add pkg/tools/kios/commands.go
git commit -m "style(kios): refactor panduanText dengan pemisah kategori + info notif + tips token"
```

---

## Task 6: Dokumentasi env vars

**Files:**
- Modify: `deploy/env.example`

### Step 1: Tambah dua env var baru ke `env.example`

- [ ] Di `deploy/env.example`, tambahkan setelah blok `KIOS_BACKUP_CHAT`/`KIOS_BACKUP_CRON`:

```bash
# Ekonomi mode: matikan laporan + backup otomatis (notif stok & pesanan tetap jalan).
# Set "true" untuk menghemat request LLM di Groq/Gemini free tier.
KIOS_EKONOMI_MODE=

# Jam aktif WITA untuk laporan & backup otomatis. Format "HH-HH" (24 jam WITA).
# Contoh: "07-22" = hanya aktif jam 07:00–22:00, "22-06" = aktif malam s.d. subuh.
# Kosong = aktif 24 jam (default).
KIOS_JAM_AKTIF=
```

### Step 2: Jalankan test final

- [ ]
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
go test -tags goolm,stdjson ./pkg/tools/kios/... 2>&1 | tail -5
cd kios-dashboard && npx tsc --noEmit 2>&1 | head -5; echo "TS: $?"
```

Expected: Go `ok`, TS `exit: 0`.

### Step 3: Commit Task 6

- [ ]
```bash
git add deploy/env.example
git commit -m "docs(deploy): dokumentasikan KIOS_EKONOMI_MODE + KIOS_JAM_AKTIF di env.example"
```

---

## Self-Review

### Spec Coverage

| Requirement | Task |
|---|---|
| Toggle notif stok via `/notif stok on\|off` | Task 4 |
| Toggle notif pesanan via `/notif pesanan on\|off` | Task 4 |
| Status notif via `/notif` | Task 4 |
| `NotifPesananEnabled` di KiosConfig + default true | Task 1 |
| Guard `tryNotifyOrders` + `tryNotifyPendingPileup` | Task 3 |
| `KIOS_EKONOMI_MODE` matikan laporan + backup | Task 2 + 3 |
| `KIOS_JAM_AKTIF` time window | Task 2 |
| `IsJamAktif` overnight support | Task 2 |
| Pemisah kategori `panduanText` | Task 5 |
| TS mirror `notif_pesanan_enabled` | Task 1 |
| Dokumentasi env.example | Task 6 |

### Placeholder Scan

Tidak ada TBD/TODO/placeholder dalam plan ini.

### Type Consistency

- `NotifPesananEnabled bool` didefinisikan Task 1, dipakai Task 3 (notif.go guard) dan Task 4 (command toggle) — konsisten.
- `IsEkonomiMode()` didefinisikan Task 2, dipakai Task 3 (report.go, backup.go) — konsisten.
- `IsServiceAktif()` didefinisikan Task 2, tersedia untuk future use — konsisten.
- `isJamAktifAt(hour int)` — fungsi internal (lowercase), hanya dipakai dalam package dan test — konsisten.
- `cfg.NotifPesananEnabled` di command handler (Task 4) menggunakan `store.GetConfig(ctx)` dan `store.SaveConfig(ctx, cfg)` — pola sama dengan handler lain di commands.go.
