# Mini Kios Desa — Design Spec
Date: 2026-06-04  
Status: Approved  
Reference: CLAUDENEW.md + kios-picoclaw patterns

---

## Tujuan

Membangun sistem manajemen kios desa (Rote Ndao, NTT) dari nol di `~/Publik/project/minikios`.  
Target: bersih, pragmatis, mudah ditambah fitur baru tanpa menyentuh yang lain.

---

## Arsitektur Keseluruhan

```
Telegram Bot
    │
    ▼ (long polling, 24/7)
PicoClaw Agent ──── Railway Cloud (Docker)
    │     │
    │     ├── AI Models: Groq (utama) → Gemini (fallback)
    │     └── GitHub Repo (private) → auto-deploy Railway
    │
    ▼
Upstash Redis (database utama)
    │
    └── Dashboard Kios (lokal, React + Vite, port 5173)
```

**Prinsip keras:**
- Railway = satu-satunya runtime agent
- Upstash Redis = satu-satunya database
- Dashboard lokal konek langsung ke Upstash via REST API
- Private GitHub repository

---

## Struktur Direktori

```
minikios-desa/
├── agent/
│   ├── AGENT.md                  ← persona + aturan agent
│   ├── workspace/
│   │   └── harga.md              ← daftar harga (edit = update harga agent)
│   ├── config/
│   │   └── kios.json             ← model list, channels, redis config
│   ├── scripts/
│   │   └── entrypoint.sh         ← render config dari env, jalankan picoclaw
│   ├── Dockerfile
│   └── .env.example
│
├── dashboard/
│   ├── src/
│   │   ├── lib/
│   │   │   ├── redis.js          ← SATU-SATUNYA akses Redis (analog store.go)
│   │   │   ├── auth.js           ← session + role check
│   │   │   └── format.js         ← Rupiah, WITA, ID generator
│   │   ├── pages/
│   │   │   ├── Transaksi.jsx     ← halaman utama kasir
│   │   │   ├── Pelanggan.jsx
│   │   │   ├── Utang.jsx
│   │   │   ├── Stok.jsx
│   │   │   ├── Supplier.jsx
│   │   │   ├── Rekap.jsx
│   │   │   ├── Users.jsx         ← admin only
│   │   │   ├── Backup.jsx        ← admin only
│   │   │   └── Setup.jsx         ← hanya saat Redis kosong
│   │   ├── components/
│   │   │   ├── Sidebar.jsx
│   │   │   ├── Header.jsx
│   │   │   └── Toast.jsx
│   │   └── App.jsx               ← routing + protected routes
│   ├── index.html
│   ├── vite.config.js            ← host: true (akses dari HP)
│   └── package.json
│
├── docs/
│   ├── SETUP.md
│   ├── DEPLOY_RAILWAY.md
│   └── CARA_PAKAI.md
│
├── .github/
│   └── workflows/deploy.yml      ← push main → Railway auto-deploy
│
├── .gitignore
└── README.md
```

---

## Redis Key Schema

Satu-satunya referensi schema. Semua operasi via `redis.js`.

```
PREFIX: "kios:"

Transaksi:
  kios:transaksi:idx:{YYYYMMDD}   LIST   uuid transaksi per hari
  kios:transaksi:{uuid}           HASH   {jenis, pelanggan_id, nominal, harga,
                                          admin, total, bayar, kembalian,
                                          catatan, status, kasir, created_at}
  kios:rekap:{YYYYMMDD}           HASH   {total_omzet, total_trx, breakdown_json}

Pelanggan:
  kios:pelanggan:idx              SET    semua id pelanggan
  kios:pelanggan:{id}             HASH   {nama, hp, created_at}

Utang:
  kios:utang:pembeli:{id}         HASH   {pelanggan_id, total, status,
                                          jatuh_tempo, updated_at}
  kios:utang:supplier:{id}        HASH   {supplier_id, total, status,
                                          keterangan, updated_at}

Stok:
  kios:stok:{produk_id}           HASH   {nama, stok, minimum, updated_at}

Harga:
  kios:harga                      HASH   {produk_id → JSON harga object}

Supplier:
  kios:supplier:idx               SET    semua id supplier
  kios:supplier:{id}              HASH   {nama, hp, produk, created_at}

User:
  kios:user:{username}            HASH   {role, password_hash, nama, last_login}

Config:
  kios:config                     HASH   {nama_kios, versi}
```

---

## Panduan Extensibility Redis

### Tambah field baru ke entitas yang ada
Contoh: tambah "email" ke pelanggan
1. Edit `redis.js` → method `simpanPelanggan()` → tambah field di HSET
2. `getPelanggan()` otomatis ikut (pakai HGETALL)
3. Edit halaman yang menampilkan data pelanggan
✅ Tidak perlu migrasi — Redis HASH fleksibel

### Tambah kategori transaksi baru
Contoh: tambah "Isi Bensin"
1. Edit `agent/workspace/harga.md` → tambah tabel harga baru
2. Edit `redis.js` → tambah ke konstanta `JENIS_TRANSAKSI` (string, bukan angka)
3. Edit `Transaksi.jsx` → tambah opsi selector
✅ Rekap otomatis support karena group-by string

### Tambah key schema baru
Contoh: sistem reminder utang
1. Dokumentasikan di bagian komentar "Key Schema" di atas `redis.js`
2. Tambah method baru di `redis.js`
✅ Tidak ada yang perlu diubah di tempat lain

### Konvensi penamaan key
```
kios:{entitas}:{id}          → data utama
kios:{entitas}:idx           → index (SET) semua id
kios:{entitas}:idx:{param}   → index per parameter (misal: per tanggal)
```

---

## Slash Commands Bot Telegram

Commands diproses **sebelum AI** (rule-based, 0 token LLM).

### Commands bawaan picoclaw
```
/start    → salam + panduan singkat
/help     → daftar semua perintah
/clear    → hapus konteks percakapan
```

### Commands kios (ikuti pola commands.go kios-picoclaw)
```
/stok                           → ringkasan stok semua produk
/harga                          → daftar harga
/jual [jenis] [nominal] [hp]    → catat transaksi cepat
/pulsa [nominal] [hp]           → shortcut jual pulsa
/paket [produk] [hp]            → shortcut jual paket data
/token [nominal] [no_meter]     → shortcut token PLN
/transfer [nominal] [hp]        → shortcut transfer dana
/utang                          → daftar utang aktif
/laporan                        → rekap hari ini
/pelanggan [nama/hp]            → cari atau tambah pelanggan
/lunas [id_utang]               → tandai utang lunas
```

### Cara tambah command baru
1. Tambah handler di `agent/AGENT.md` bagian "Slash Commands"
2. Dokumentasikan di `docs/CARA_PAKAI.md`
✅ Commands di satu tempat (AGENT.md), mudah dicari dan diubah

---

## Dashboard Pages

| Route | Halaman | Akses | Fungsi utama |
|-------|---------|-------|-------------|
| `/transaksi` | Transaksi | semua | Form kasir + daftar hari ini |
| `/pelanggan` | Pelanggan | semua | Daftar + detail + riwayat |
| `/utang` | Utang | semua | Pembeli & supplier, tandai lunas |
| `/stok` | Stok | semua | Stok produk fisik, alert kritis |
| `/supplier` | Supplier | semua | Manajemen + catat pembelian |
| `/rekap` | Rekap | semua | Laporan harian/range, export CSV |
| `/users` | Users | admin | CRUD user + log aktivitas |
| `/backup` | Backup | admin | Export/import JSON |
| `/setup` | Setup | publik | Setup awal (hanya saat Redis kosong) |

---

## Auth & Role

| Aksi | kasir | admin |
|------|:-----:|:-----:|
| Catat transaksi | ✅ | ✅ |
| Lihat utang, tandai lunas | ✅ | ✅ |
| Update stok | ✅ | ✅ |
| Lihat rekap | ✅ | ✅ |
| Manajemen user | ❌ | ✅ |
| Backup/restore | ❌ | ✅ |
| Update harga | ❌ | ✅ |

Session disimpan di `sessionStorage` (bukan localStorage, aman dari tab lain).  
Password disimpan sebagai bcrypt hash di Redis.

---

## Desain Visual Dashboard

**Palet warna:**
```css
--hijau-kios:   #16a34a   /* aksi utama, tombol catat */
--hijau-muda:   #dcfce7   /* background sukses */
--biru-info:    #2563eb   /* info, link */
--kuning-warn:  #d97706   /* stok menipis, perhatian */
--merah-danger: #dc2626   /* utang jatuh tempo, error */
--abu-bg:       #f8fafc   /* background halaman */
--abu-card:     #ffffff   /* background card */
--abu-border:   #e2e8f0   /* border */
--teks-utama:   #0f172a   /* teks primer */
--teks-muted:   #64748b   /* teks sekunder */
```

**Font:** `Plus Jakarta Sans` (Google Fonts)  
**Angka rupiah:** monospace, 16px/600

---

## Environment Variables

### Agent (Railway)
```
GROQ_API_KEY=         # wajib
TELEGRAM_BOT_TOKEN=   # wajib
UPSTASH_REDIS_URL=    # wajib
UPSTASH_REDIS_TOKEN=  # wajib
GEMINI_API_KEY=       # opsional, fallback LLM
KIOS_NAMA=            # nama kios tampil di greeting
KIOS_ADMIN_IDS=       # Telegram user IDs yang jadi admin (koma)
```

### Dashboard (lokal)
```
VITE_UPSTASH_REDIS_URL=
VITE_UPSTASH_REDIS_TOKEN=
VITE_KIOS_NAMA=
VITE_KIOS_PASSWORD_ADMIN=admin123   # ganti setelah login pertama
VITE_KIOS_PASSWORD_KASIR=kasir123   # ganti setelah login pertama
```

---

## CI/CD

GitHub Actions: push ke `main` yang menyentuh folder `agent/` → trigger Railway deploy otomatis.  
Railway menggunakan `agent/Dockerfile` sebagai build context.

---

## Checklist Build (untuk 5 agent paralel)

- [ ] **Agent 1** — Scaffold direktori + agent/ (AGENT.md, harga.md, kios.json, entrypoint.sh, Dockerfile, .env.example)
- [ ] **Agent 2** — dashboard/src/lib/ (redis.js lengkap + auth.js + format.js)
- [ ] **Agent 3** — dashboard/src/pages/ (Transaksi, Pelanggan, Utang, Stok)
- [ ] **Agent 4** — dashboard/src/pages/ (Supplier, Rekap, Users, Backup, Setup) + components/ (Sidebar, Header, Toast)
- [ ] **Agent 5** — App.jsx + vite.config.js + package.json + docs/ + .github/workflows/ + README.md + .gitignore
