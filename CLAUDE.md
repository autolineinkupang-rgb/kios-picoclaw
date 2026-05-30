# kios-picoclaw — CLAUDE.md

Panduan ini untuk Claude Code agar memahami proyek ini sebelum mulai bekerja.

## Apa Proyek Ini

**kios-picoclaw** adalah fork dari [sipeed/picoclaw](https://github.com/sipeed/picoclaw) yang dikustomisasi menjadi **asisten AI kios desa** untuk Rote Ndao, NTT. Sistem terdiri dari dua komponen utama:

1. **Bot Telegram (Go)** — picoclaw core + kios tools, deploy di Railway
2. **Dashboard Web (Next.js)** — admin panel + storefront, deploy di Vercel (`kios-dashboard/`)

Data persisten di **Upstash Redis** (Railway FS bersifat ephemeral).

---

## Toolchain & Build

Go **tidak** ada di PATH non-interaktif. Selalu prefix perintah Go:

```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
```

| Perintah | Keterangan |
|----------|-----------|
| `make build` | Build binary ke `build/picoclaw` |
| `make install` | Install ke `~/.local/bin/picoclaw` |
| `go test ./pkg/tools/kios/...` | Unit test kios tools |
| `make test` | Semua test |
| `cd kios-dashboard && npm run dev` | Dev server dashboard (port 3000) |
| `cd kios-dashboard && npm run build` | Build dashboard |
| `cd kios-dashboard && npm run typecheck` | Type check TypeScript |

Dependensi Go: `make deps`. Dependensi dashboard: `npm install` di `kios-dashboard/`.

---

## Struktur Direktori

```
kios-picoclaw/
├── pkg/tools/kios/          ← Semua kios tools (Go)
│   ├── store.go             ← Redis data layer + struct Go
│   ├── register.go          ← AllTools() — factory semua tools
│   ├── stok.go              ← Tool kios_stok
│   ├── kasir.go             ← Tool kios_kasir
│   ├── laporan.go           ← Tool kios_laporan
│   ├── harga.go             ← Tool kios_harga
│   ├── supplier.go          ← Tool kios_supplier
│   ├── promo.go             ← Tool kios_promo
│   ├── pustaka.go           ← Tool kios_pustaka
│   ├── pasar.go             ← Tool kios_pasar
│   ├── belajar.go           ← Tool kios_belajar
│   ├── user.go              ← Tool kios_user (RBAC)
│   ├── upload.go            ← Tool kios_import_upload
│   ├── commands.go          ← Slash commands (/stok, /jual, dll.)
│   ├── notif.go             ← Notifikasi stok menipis + pesanan
│   └── seed.go              ← Migrasi data CSV lama
├── kios-dashboard/          ← Next.js 15 dashboard
│   ├── src/app/             ← App Router pages
│   │   ├── (app)/           ← Halaman admin (auth required)
│   │   │   ├── dashboard/   ← KPI utama
│   │   │   ├── produk/      ← CRUD produk
│   │   │   ├── kasir/       ← Form kasir
│   │   │   ├── laporan/     ← Laporan penjualan
│   │   │   ├── penjualan/   ← Riwayat transaksi
│   │   │   ├── suplier/     ← Manajemen supplier
│   │   │   ├── pesanan/     ← Inbox pesanan online
│   │   │   ├── pengguna/    ← Manajemen user
│   │   │   ├── impor/       ← Import Excel/CSV
│   │   │   └── pengaturan/  ← Konfigurasi kios
│   │   ├── toko/            ← Storefront publik pembeli
│   │   ├── mall/            ← Halaman mall publik
│   │   └── login/           ← Login Telegram + kode
│   ├── src/lib/
│   │   ├── types.ts         ← TypeScript mirror dari Go structs
│   │   ├── kios.ts          ← Redis data-access functions
│   │   └── redis.ts         ← Upstash Redis client
│   └── src/components/      ← UI components
├── workspace/
│   ├── AGENT.md             ← Persona + instruksi agent kios
│   ├── SOUL.md              ← Karakter Kios Cerdas
│   └── skills/
│       └── kios-koperasi/SKILL.md  ← Skill yang di-load picoclaw
├── cmd/
│   ├── kios-seed/           ← Import data CSV lama ke Redis
│   ├── kios-import/         ← Import Excel/CSV bulk
│   └── gen-templates/       ← Generate file template Excel
├── deploy/entrypoint.sh     ← Docker entrypoint (render config.json)
├── templates/               ← Template Excel untuk import data
├── Dockerfile               ← Multi-stage Go → Alpine
├── railway.json             ← Railway deploy config
├── KIOS_BUILD_SPEC.md       ← Spesifikasi build lengkap
├── DEPLOY-RAILWAY.md        ← Panduan deploy Railway
└── PERINTAH.md              ← Panduan perintah untuk pengguna kios
```

---

## Tool Interface (picoclaw)

Semua kios tools implement interface `toolshared.Tool` dari `pkg/tools/shared/base.go`:

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any   // JSON Schema object
    Execute(ctx context.Context, args map[string]any) *ToolResult
}
```

Lihat `pkg/tools/cron.go` sebagai referensi implementasi lengkap.
Helper `ToolResult` ada di `pkg/tools/shared/result.go`.
Tools di-register di `pkg/agent/agent_init.go` via `AllTools(store)`.

---

## Redis Data Model (Upstash)

Koneksi via env `UPSTASH_REDIS_URL` (format `rediss://...`).

| Key | Tipe Redis | Isi |
|-----|-----------|-----|
| `kios:produk` | HASH | field=id, value=JSON `Produk` |
| `kios:transaksi` | LIST | JSON `Transaksi` per transaksi (RPUSH) |
| `kios:pembelian` | LIST | JSON `Pembelian` per restock |
| `kios:price_history` | LIST | JSON `PriceHistory` per ubah harga |
| `kios:shift` | STRING | JSON `Shift` (current shift state) |
| `kios:users` | HASH | field=telegram_id, value=JSON `UserKios` |
| `kios:supplier` | HASH | field=id, value=JSON `Supplier` |
| `kios:promo` | HASH | field=id, value=JSON promo |
| `kios:config` | HASH | `KiosConfig` (konfigurasi kios) |
| `kios:seq:trx` | STRING | Counter ID transaksi (INCR → TRX-0001) |
| `kios:seq:pem` | STRING | Counter ID pembelian |
| `kios:seq:phg` | STRING | Counter ID price history |

TypeScript mirror dari semua struct ada di `kios-dashboard/src/lib/types.ts`.
**Jangan ubah field name** tanpa update Go + TypeScript sekaligus.

---

## RBAC (Role-Based Access Control)

| Aksi | kasir | owner |
|------|:-----:|:-----:|
| Jual, restock, cek harga | ✅ | ✅ |
| Update harga jual | ✅ | ✅ |
| Tambah produk baru | ❌ | ✅ |
| Hapus produk | ❌ | ✅ |
| Batalkan transaksi | ❌ | ✅ |
| Set stok manual | ❌ | ✅ |
| Kelola user (kios_user) | ❌ | ✅ |
| Ubah konfigurasi belajar | ❌ | ✅ |

Role diambil dari `kios:users` berdasarkan Telegram ID. Default role: env `KIOS_DEFAULT_ROLE` (default `owner`).
Owner permanen bisa di-set via env `KIOS_OWNER_IDS` (comma-separated Telegram IDs).

---

## Environment Variables

| Variabel | Wajib | Keterangan |
|----------|:-----:|-----------|
| `UPSTASH_REDIS_URL` | ✅ | Koneksi Redis (`rediss://`) |
| `TELEGRAM_BOT_TOKEN` | ✅ | Token bot Telegram |
| `KIOS_ALLOW_FROM` | ✅ | Whitelist Telegram IDs (koma) |
| `GROQ_API_KEY` | ✅ | LLM utama (Groq / Llama) |
| `GEMINI_API_KEY` | — | Fallback LLM |
| `ANTHROPIC_API_KEY` | — | Fallback LLM (Claude) |
| `KIOS_DEFAULT_ROLE` | — | Default role (`owner`/`kasir`) |
| `KIOS_OWNER_IDS` | — | Owner permanen (tidak bisa dikunci) |
| `KIOS_REPORT_CHAT` | — | Chat ID laporan harian otomatis |
| `KIOS_REPORT_CRON` | — | Jadwal laporan (cron, TZ Makassar) |
| `KIOS_BACKUP_CHAT` | — | Chat ID backup otomatis |
| `KIOS_BACKUP_CRON` | — | Jadwal backup (default `0 22 * * *`) |
| `KIOS_DASHBOARD_URL` | — | Base URL dashboard (tanpa trailing slash) |
| `KIOS_SERVICE_SECRET` | — | HMAC secret (harus sama dg dashboard) |
| `KIOS_TEMPLATE_DIR` | — | Direktori template Excel (default `templates`) |

Dashboard (Vercel) butuh tambahan:
- `UPSTASH_REDIS_URL` — sama dengan bot
- `KIOS_SERVICE_SECRET` — untuk validasi request dari bot

---

## Deploy

### Bot (Railway)
1. Push ke GitHub
2. Railway → New Project → Deploy from GitHub → pilih repo
3. Isi Variables (lihat di atas)
4. Auto-detect `railway.json` + `Dockerfile`
5. Health check: `GET /health` (port 18790, atau `$PORT` dari Railway)

Build Docker: multi-stage `golang:1.25-alpine` → `alpine:3.23`.
Entrypoint: `deploy/entrypoint.sh` (render `config.json` dari env, jalankan `picoclaw gateway`).

### Dashboard (Vercel)
- Root directory: `kios-dashboard`
- Framework: Next.js
- Env vars: `UPSTASH_REDIS_URL`, `KIOS_SERVICE_SECRET`, dll.

---

## Konvensi Koding

- Go: ikuti `gofmt`, file < 500 baris
- Test: table-driven, gunakan `miniredis` untuk mock Redis
- Dashboard: TypeScript strict, Tailwind CSS, Server Actions untuk mutasi
- Bahasa komentar kode: English; UI/notif/dokumen: Bahasa Indonesia
- Jangan commit secrets (`.env`, `config.json` dengan API key)
- Kios tools bersifat **additive** — jangan break picoclaw upstream build

---

## Tools Kios (ringkasan)

| Tool | Nama Go | File |
|------|---------|------|
| `kios_stok` | `StokTool` | `stok.go` |
| `kios_kasir` | `KasirTool` | `kasir.go` |
| `kios_laporan` | `LaporanTool` | `laporan.go` |
| `kios_harga` | `HargaTool` | `harga.go` |
| `kios_supplier` | `SupplierTool` | `supplier.go` |
| `kios_promo` | `PromoTool` | `promo.go` |
| `kios_pustaka` | `PustakaTool` | `pustaka.go` |
| `kios_pasar` | `PasarTool` | `pasar.go` |
| `kios_belajar` | `BelajarTool` | `belajar.go` |
| `kios_user` | `UserTool` | `user.go` |
| `kios_import_upload` | `UploadTool` | `upload.go` |
| `kios_restore` | `RestoreTool` | `restore.go` |

Semua di-register via `AllTools(store)` di `register.go`.

---

## Fitur Khusus

- **Slash commands** — `/stok`, `/jual`, `/laporan`, dll. berjalan tanpa AI (langsung tool)
- **Notifikasi otomatis** — stok menipis + pesanan baru (loop 2 menit via `notif.go`)
- **Laporan harian** — dikirim via cron ke Telegram + integrasi dashboard summary
- **Backup/restore** — export JSON semua data, restore dari file
- **Import massal** — Excel/CSV via Telegram (upload file) atau CLI (`kios-import`)
- **Storefront publik** — `kios-dashboard/toko` untuk pembeli (tanpa auth)
- **Pesanan online** — pembeli order lewat toko web, owner konfirmasi di dashboard
