# Design: Ekonomi Mode + Toggle Notifikasi + Kategori Panduan

**Tanggal:** 2026-06-04
**Branch target:** feat/ekonomi-notif-toggle

---

## Latar Belakang

Pengguna menggunakan Groq/Gemini free tier yang cepat habis. Dibutuhkan:
1. Toggle on/off notif stok & pesanan via Telegram (slash command, 0 token)
2. Mematikan laporan & backup otomatis via env var (Railway)
3. Panduan `/panduan` yang lebih mudah dibaca dengan pemisah kategori

---

## Analisis Token Consumption

| Fitur | Pakai LLM? | Frekuensi |
|---|---|---|
| Notif stok/pesanan | ❌ 0 token | Polling 2 menit |
| Laporan harian | ⚠️ Mungkin 1 request | 1×/hari |
| Backup otomatis | ❌ 0 token | 1×/hari |
| Chat user ke bot | ✅ 1+ request/pesan | Per interaksi |

---

## Bagian 1: Toggle Notifikasi via Telegram

### KiosConfig — field baru

Tambah field `NotifPesananEnabled` ke `KiosConfig` di `store.go`:

```go
NotifPesananEnabled bool `json:"notif_pesanan_enabled"` // default true
```

Default: `true` (aktif). Field `NotifEnabled` yang sudah ada untuk stok tetap dipakai.

### Slash command `/notif`

Command baru di `commands.go` dengan sub-commands:

```
/notif               → tampilkan status semua notifikasi
/notif stok on       → aktifkan notif stok menipis
/notif stok off      → matikan notif stok menipis
/notif pesanan on    → aktifkan notif pesanan baru & menumpuk
/notif pesanan off   → matikan notif pesanan baru & menumpuk
```

**RBAC:** owner-only. Kasir tidak bisa ubah pengaturan notif.

**Storage:** disimpan di Redis via `SetConfig` — bertahan restart.

**Output `/notif` (status):**
```
🔔 Status Notifikasi

Stok menipis : ✅ Aktif  (jam 07:00 WITA)
Pesanan baru : ❌ Nonaktif

Ubah dengan /notif stok on|off atau /notif pesanan on|off
```

### Perubahan `notif.go`

- `tryNotify()`: sudah cek `cfg.NotifEnabled` — tidak perlu perubahan
- `tryNotifyOrders()`: tambah guard `if !cfg.NotifPesananEnabled { return }`
- `tryNotifyPendingPileup()`: tambah guard yang sama

---

## Bagian 2: Ekonomi Mode via Env Var

### Env vars baru

| Var | Default | Contoh | Efek |
|---|---|---|---|
| `KIOS_EKONOMI_MODE` | kosong | `true` | Matikan laporan + backup otomatis |
| `KIOS_JAM_AKTIF` | kosong (24 jam) | `07-22` | Laporan/backup hanya jalan jam 07-22 WITA |

### File baru: `pkg/tools/kios/service_gate.go`

Helper fungsi murni (no side effects, mudah di-test):

```go
// IsLaporanAktif returns false jika KIOS_EKONOMI_MODE=true.
func IsLaporanAktif() bool

// IsJamAktif checks apakah jam WITA saat ini dalam window KIOS_JAM_AKTIF.
// Format: "HH-HH", mendukung overnight (e.g. "22-06").
// Jika env kosong → selalu true.
func IsJamAktif() bool

// IsServiceAktif combines both: false jika ekonomi mode atau di luar jam aktif.
func IsServiceAktif() bool
```

### Perubahan `report.go`

`EnsureDailyReportJob`: tambah guard di awal:
```go
if !IsLaporanAktif() {
    logger.Infof("kios: laporan harian dinonaktifkan (KIOS_EKONOMI_MODE)")
    return nil
}
```

### Perubahan `backup.go`

`EnsureBackupJob`: tambah guard yang sama.

---

## Bagian 3: Pemisah Kategori di `/panduan`

Refactor `panduanText` di `commands.go` dengan emoji separator per kategori:

```
📖 Panduan Kios Cerdas

━━━ 🛒 KASIR & TRANSAKSI ━━━
/stok      — cek stok / cari produk
/jual      — catat penjualan
/laporan   — ringkasan penjualan hari ini
...

━━━ 💰 HARGA & INFO ━━━
/harga     — lihat harga jual & modal
/promo     — daftar promo aktif
/pasar     — bandingkan harga vs pasar
...

━━━ 📦 STOK KHUSUS ━━━
/pulsa     — cek/jual pulsa
/bensin    — cek/jual bensin
...

━━━ 🔔 NOTIFIKASI (Owner) ━━━
/notif     — status & toggle notifikasi stok/pesanan

━━━ ⚙️ OWNER ━━━
/backup    — export data JSON
/template  — download template Excel
/isipulsa  — top-up modal pulsa
/isibensin — restock bensin
...

━━━ 💬 TANYA AI ━━━
Ketik bebas: "laporan minggu ini", "tambah produk..."
```

---

## File yang Diubah

| File | Aksi |
|---|---|
| `pkg/tools/kios/store.go` | Tambah `NotifPesananEnabled` ke `KiosConfig` |
| `pkg/tools/kios/service_gate.go` | **BARU** — helper ekonomi mode + jam aktif |
| `pkg/tools/kios/service_gate_test.go` | **BARU** — test IsJamAktif (overnight, normal) |
| `pkg/tools/kios/notif.go` | Guard `NotifPesananEnabled` di tryNotifyOrders + tryNotifyPendingPileup |
| `pkg/tools/kios/commands.go` | Command `/notif` + refactor `panduanText` dengan kategori |
| `pkg/tools/kios/report.go` | Guard `IsLaporanAktif()` di EnsureDailyReportJob |
| `pkg/tools/kios/backup.go` | Guard `IsLaporanAktif()` di EnsureBackupJob |
| `kios-dashboard/src/lib/types.ts` | Tambah `notif_pesanan_enabled?: boolean` ke `KiosConfig` |
| `deploy/env.example` | Dokumentasikan `KIOS_EKONOMI_MODE` + `KIOS_JAM_AKTIF` |
| `CLAUDE.md` | Update tabel env vars |

---

## Contoh Penggunaan di Railway

```bash
# Matikan laporan + backup (hemat 1-2 request/hari)
KIOS_EKONOMI_MODE=true

# ATAU: batasi jam aktif (laporan/backup hanya jalan jam 07-22)
KIOS_JAM_AKTIF=07-22

# Dari bot Telegram:
/notif stok off      # matikan notif stok
/notif pesanan off   # matikan notif pesanan
/notif               # cek status
```

---

## Self-Review

- ✅ Tidak ada placeholder/TBD
- ✅ Notif stok/pesanan bisa dikontrol via bot (0 token)
- ✅ Laporan/backup dikontrol via env var (Railway restart)
- ✅ Default semua aktif — tidak ada perubahan perilaku untuk existing deployment
- ✅ `IsJamAktif` mendukung overnight range (22-06)
- ✅ `NotifPesananEnabled` default `true` via `GetConfig()` default value
