# agents.md — Panduan Agent & Skill kios-picoclaw

Dokumen ini menjelaskan **agent AI** yang berjalan di sistem kios-picoclaw: persona, tools, skill, dan cara kerjanya.

---

## 1. Agent Utama: Kios Cerdas

### Identitas

- **Nama**: Kios Cerdas 🛒
- **Persona file**: `workspace/AGENT.md` (instruksi) + `workspace/SOUL.md` (karakter)
- **Bahasa**: Bahasa Indonesia, santai, panggil pengguna "kak"
- **Konteks**: Asisten kios desa di Rote Ndao, NTT
- **Zona waktu**: WITA (UTC+8)
- **Format uang**: `Rp 15.000`

### Cara Kerja

Agent dijalankan oleh picoclaw melalui `picoclaw gateway`. Setiap pesan Telegram dari user yang ada di whitelist (`KIOS_ALLOW_FROM`) masuk ke agent loop. Agent:

1. Membaca pesan user
2. Memilih action (slash command langsung OR tool call via LLM)
3. Memanggil kios tools untuk data nyata dari Redis
4. Membalas singkat dan langsung di Telegram

Agent **tidak boleh mengarang angka** — semua stok, harga, dan laba harus dari tools.

### LLM Provider

| Provider | Model | Variabel |
|----------|-------|---------|
| Groq (utama) | `meta-llama/llama-4-scout-17b-16e-instruct` | `GROQ_API_KEY` |
| Gemini (fallback) | `gemini-2.0-flash` | `GEMINI_API_KEY` |
| Anthropic (opsional) | `claude-sonnet-4-6` | `ANTHROPIC_API_KEY` |

---

## 2. Tools Agent

Semua tools berada di `pkg/tools/kios/`. Setiap tool implement interface `toolshared.Tool`.

### kios_stok

**File**: `stok.go` | **Struct**: `StokTool`

Kelola stok produk.

| Action | Parameter | Keterangan |
|--------|-----------|-----------|
| `cek` | — | Daftar semua produk |
| `cari` | `produk` | Cari produk (exact id / substring nama) |
| `jual` | `produk`, `qty`, `metode?` | Catat penjualan, kurangi stok |
| `tambah` | `produk`, `qty`, `harga?`, `supplier?`, `auto_create?` | Restock, bisa buat produk baru |
| `tambah_produk` | `nama`, `harga_jual`, `stok`, ... | Daftarkan produk baru |
| `hapus` | `produk` | Hapus produk (owner only) |
| `set_stok` | `produk`, `stok_baru` | Atur stok manual (owner only) |
| `update_exp` | `produk`, `exp_date` | Ubah tanggal kedaluwarsa |
| `batalkan_tx` | `id` | Batalkan transaksi, kembalikan stok |
| `stok_menipis` | — | Produk dengan stok ≤ stok_minimum |

### kios_kasir

**File**: `kasir.go` | **Struct**: `KasirTool`

Kasir dengan struk dan manajemen shift.

| Action | Parameter | Keterangan |
|--------|-----------|-----------|
| `jual` | `produk`, `qty`, `metode?`, `bayar?` | Jual + cetak struk + kembalian |
| `buka_shift` | `kasir`, `saldo_awal` | Buka shift kasir |
| `tutup_shift` | `saldo_akhir` | Tutup shift, hitung omzet |
| `status_shift` | — | Cek status shift aktif |

Struk berisi: header, lokasi Rote Barat Laut, items, total, bayar, kembalian, TRX-xxxx, waktu WITA.

### kios_laporan

**File**: `laporan.go` | **Struct**: `LaporanTool`

Laporan penjualan dan analisis bisnis.

| Action | Parameter | Keterangan |
|--------|-----------|-----------|
| `ringkas` | `tanggal?` | Ringkasan harian |
| `mingguan` | — | Ringkasan minggu ini |
| `bulanan` | — | Ringkasan bulan ini |
| `laba` | `periode` | Laba (omzet - modal) per periode |
| `riwayat` | `periode` | 20 transaksi terakhir |
| `terlaris` | `periode`, `top?` | Top N produk terlaris |
| `riwayat_harga` | `produk?` | Riwayat perubahan harga |

Laba = Σ(total penjualan) − Σ(qty × harga_beli per produk).

### kios_harga

**File**: `harga.go` | **Struct**: `HargaTool`

Manajemen harga dan estimasi margin.

| Action | Parameter | Keterangan |
|--------|-----------|-----------|
| `cek` | `produk` | Lihat harga beli & jual saat ini |
| `update` | `produk`, `harga_jual`, `harga_beli?` | Update harga (dicatat ke price_history) |
| `estimasi` | `produk`, `harga_beli_baru?` | Estimasi margin 10/15/20/25% |
| `prediksi` | `produk` | Prediksi tren harga (butuh ≥2 data) |

### kios_supplier

**File**: `supplier.go` | **Struct**: `SupplierTool`

Kelola data supplier dan bandingkan harga beli.

| Action | Parameter | Keterangan |
|--------|-----------|-----------|
| `daftar` | — | Daftar semua supplier |
| `cari` | `nama` | Cari supplier |
| `tambah` | `nama`, `kontak`, ... | Tambah supplier baru |
| `update` | `id`, ... | Update info supplier |
| `hapus` | `id` | Hapus supplier |
| `banding` | `produk` | Bandingkan harga beli antar supplier |
| `set_harga` | `supplier_id`, `produk_id`, `harga` | Set harga beli dari supplier tertentu |

### kios_promo

**File**: `promo.go` | **Struct**: `PromoTool`

Kelola promo dan diskon.

| Action | Parameter | Keterangan |
|--------|-----------|-----------|
| `buat` | `nama`, `tipe`, `nilai`, `berlaku_sampai?`, ... | Buat promo baru |
| `daftar` | — | Daftar promo aktif |
| `cek` | `id` | Detail promo |
| `hapus` | `id` | Hapus promo |

### kios_pustaka

**File**: `pustaka.go` | **Struct**: `PustakaTool`

Basis pengetahuan kios (sumber info + URL safety check).

| Action | Parameter | Keterangan |
|--------|-----------|-----------|
| `tambah` | `judul`, `isi`, `url?` | Simpan info/sumber |
| `cari` | `query` | Cari dalam pustaka |
| `daftar` | — | Daftar semua entri |
| `hapus` | `id` | Hapus entri |
| `cek_url` | `url` | Cek keamanan URL (anti-malware/phishing) |

### kios_pasar

**File**: `pasar.go` | **Struct**: `PasarTool`

Intelijen harga pasar — bandingkan harga kios vs pasar.

| Action | Parameter | Keterangan |
|--------|-----------|-----------|
| `set_pasar` | `produk`, `harga`, `sumber?` | Catat harga pasar |
| `analisa` | `produk?` | Analisa harga kios vs pasar |
| `sumber` | — | Daftar sumber harga pasar tersimpan |

### kios_belajar

**File**: `belajar.go` | **Struct**: `BelajarTool`

Memori belajar adaptif — alias produk, shortcut, kebiasaan.

| Action | Parameter | Keterangan |
|--------|-----------|-----------|
| `simpan` | `kunci`, `nilai` | Simpan alias / shortcut |
| `cari` | `kunci` | Cari alias |
| `daftar` | — | Semua entri memori |
| `hapus` | `kunci` | Hapus entri |
| `habit_track` | `event`, `data` | Catat kebiasaan (jam ramai, produk laris) |
| `habit` | `tipe?` | Insight dari kebiasaan tercatat |
| `config_get` | — | Baca konfigurasi kios |
| `config_set` | `key`, `value` | Ubah konfigurasi (owner only) |

Config yang bisa diatur via `config_set`: `auto_learn_enabled`, `learn_model`, `notif_enabled`, `notif_jam`, `qris_enabled`, `qris_nama`, `qris_image_url`, `wa_number`.

### kios_user

**File**: `user.go` | **Struct**: `UserTool`

Manajemen pengguna dan RBAC (owner only).

| Action | Parameter | Keterangan |
|--------|-----------|-----------|
| `daftar` | — | Daftar semua user |
| `tambah` | `telegram_id`, `nama`, `role` | Tambah user |
| `update` | `telegram_id`, `nama?`, `role?`, `aktif?` | Update user |
| `hapus` | `telegram_id` | Hapus user |
| `cek` | `telegram_id` | Cek info user |

### kios_import_upload

**File**: `upload.go` | **Struct**: `UploadTool`

Import file Excel/CSV yang diunggah langsung di Telegram.

Dipanggil otomatis saat ada file lampiran di chat + permintaan import.
Mendukung format: `.xlsx`, `.csv`.
Bisa import: produk, supplier.

### kios_restore

**File**: `restore.go` | **Struct**: `RestoreTool`

Restore data dari file backup JSON.

Dipanggil saat user mengirim file `.json` + minta restore.
Menimpa semua data yang ada — tampilkan preview ringkasan, minta konfirmasi `"ya, restore paksa"` sebelum eksekusi.

---

## 3. Slash Commands (tanpa AI)

Perintah ini berjalan langsung tanpa melewati LLM — lebih cepat dan selalu tersedia.
Diimplementasikan di `pkg/tools/kios/commands.go`.

| Perintah | Aksi |
|----------|------|
| `/stok [nama]` | Cek stok semua atau cari produk |
| `/produk [nama]` | Daftar / detail produk |
| `/harga <nama>` | Cek harga jual |
| `/jual <produk> <jml>` | Jual + struk |
| `/jualmassal <produk> <jml>, ...` | Jual beberapa sekaligus |
| `/menipis` | Produk hampir habis |
| `/shift` | Status shift kasir |
| `/laporan` | Ringkasan hari ini |
| `/promo` | Promo aktif |
| `/pasar [produk]` | Harga kios vs pasar |
| `/suplier [nama\|banding <produk>]` | Info supplier |
| `/qris` | Tampilkan QR pembayaran |
| `/template` | Download template Excel |
| `/backup` | Export data JSON (owner) |

---

## 4. Skill yang Di-load picoclaw

Skill dimuat dari file `SKILL.md` di direktori workspace secara hierarkis.

### workspace/skills/kios-koperasi/SKILL.md

Skill utama kios. Berisi:
- Instruksi persona (asisten kios desa Rote Ndao)
- Tabel mapping tools (kapan memanggil tool apa)
- Aturan RBAC
- Perilaku proaktif (cek stok setelah jual, ingatkan margin tipis)

### workspace/skills/hardware/SKILL.md

Skill untuk interaksi I2C/SPI dengan hardware Sipeed (LicheeRV Nano, MaixCAM, NanoKVM).
Tools: `i2c`, `spi`. Referensi pinout di `hardware/references/`.

### Skill lain (non-kios)

| Skill | Deskripsi |
|-------|-----------|
| `agent-browser` | Kontrol browser via agent |
| `summarize` | Rangkum konten panjang |
| `tmux` | Kontrol session tmux |
| `weather` | Info cuaca |
| `skill-creator` | Buat skill baru |

---

## 5. Layanan Background

### NotifService (`notif.go`)

Loop setiap 2 menit:
- **Stok menipis** — kirim alert ke semua owner aktif saat jam `notif_jam` WITA (bila `notif_enabled`)
- **Pesanan baru** — notifikasi ke owner saat ada pesanan `pending` masuk
- **Pesanan menumpuk** — alert bila pending > `KIOS_PENDING_ALERT_THRESHOLD` (default 5)

### Laporan Harian

Dijadwalkan via cron (`KIOS_REPORT_CRON`, default `0 18 * * *` TZ Makassar):
- Ambil summary dari `GET $KIOS_DASHBOARD_URL/api/summary` (HMAC auth)
- Format laporan + terlaris + stok kritis
- Kirim ke `KIOS_REPORT_CHAT`

### Backup Otomatis

Dijadwalkan via cron (`KIOS_BACKUP_CRON`, default `0 22 * * *`):
- Export semua data Redis ke JSON
- Kirim file ke `KIOS_BACKUP_CHAT`

---

## 6. Integrasi Dashboard ↔ Bot

Dashboard (`kios-dashboard`) dan bot berbagi data Redis yang sama.

**Komunikasi bot → dashboard**: bot baca/tulis Redis langsung.

**Komunikasi dashboard → bot**: endpoint `GET/POST /api/summary` dan `/api/pesanan` di dashboard,
dilindungi HMAC signature (`KIOS_SERVICE_SECRET`). Bot memanggil endpoint ini untuk laporan harian.

**Auth dashboard**: Login Telegram OAuth atau kode sementara (`/api/auth/code`).

---

## 7. Data Flow: Penjualan via Telegram

```
User Telegram
  │ "/jual beras 2 bayar 30000"
  ↓
picoclaw gateway (port 18790 / $PORT)
  │ channel: telegram
  ↓
Command handler (commands.go)
  │ parse → kasir.jual(produk="beras", qty=2, bayar=30000)
  ↓
kios_kasir.Execute()
  │ → cari_produk("beras") → kios:produk [HGET]
  │ → kurangi stok → kios:produk [HSET]
  │ → INCR kios:seq:trx → TRX-0042
  │ → RPUSH kios:transaksi [JSON]
  │ → format struk
  ↓
Telegram ← struk + kembalian Rp 10.000
```

---

## 8. Cara Menambah Tool Baru

1. Buat file `pkg/tools/kios/<nama>.go` dengan struct yang implement `toolshared.Tool`
2. Tambahkan factory di `register.go`:
   ```go
   func NewNamaTool(store *Store) *NamaTool { return &NamaTool{store: store} }
   ```
3. Tambahkan ke `AllTools()` di `register.go`
4. Tambahkan TypeScript types di `kios-dashboard/src/lib/types.ts` bila ada struct baru
5. Tulis unit test di `kios_test.go` atau file `_test.go` baru
6. Update `workspace/AGENT.md` dengan deskripsi tool baru
7. Update `workspace/skills/kios-koperasi/SKILL.md` dengan tabel tool baru

---

## 9. Perilaku Agent yang Diharapkan

Dari `workspace/AGENT.md` dan `workspace/SOUL.md`:

- **Singkat dulu** — jawaban satu layar HP sudah cukup
- **Data nyata** — jangan pernah mengarang stok/harga/laba
- **Proaktif ringan** — setelah jual, cek stok; kalau menipis, ingatkan singkat
- **Konfirmasi dulu** untuk aksi > Rp 500.000, hapus produk, batalkan transaksi
- **Jujur** — kalau tidak tahu atau tools error, sampaikan dengan tenang
- **Sebut TRX-xxxx** agar transaksi bisa dibatalkan bila salah
- **Margin < 10%** — beri tahu saat update harga
- **Promo aktif** — sebut sekilas saat transaksi produk yang terkena promo
