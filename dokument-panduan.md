# Panduan Pengisian ENV — kios-picoclaw

Dokumen ini menjelaskan cara mengisi **ketiga file `env.example`** untuk tiga
kebutuhan: (1) cek statis sebelum push, (2) debugging lokal, dan (3) smoke test
runtime — menjalankan bot + dashboard betulan dengan data nyata.

> Dokumen ini hanya panduan. **Tidak mengubah file atau kode yang ada.** Semua
> nilai contoh di bawah adalah placeholder — ganti dengan kredensial milikmu.

---

## 0. Fakta dasar (hasil verifikasi kode)

- **Bot Go TIDAK auto-load file `.env`.** LLM + token Telegram dibaca dari
  `config.json` (`PICOCLAW_CONFIG`, default `~/.picoclaw/config.json`).
  Sementara `UPSTASH_REDIS_URL` + semua `KIOS_*` dibaca via `os.Getenv`, jadi
  **harus ada di environment shell**.
- **Satu-satunya skrip yang merender `config.json` dari env adalah
  `deploy/entrypoint.sh`.** Cara paling konsisten menjalankan bot betulan =
  export env lalu jalankan via entrypoint (atau Docker).
- **Gateway** punya health check `GET /health` (juga `/ready`, `/reload`), pada
  port `$PORT` (default `18790`).
- **Dashboard (Next.js)** auto-load `.env.local`.
- **Data nyata** dimasukkan via CLI `kios-seed` (migrasi CSV lama) atau
  `kios-import` (Excel/CSV) — keduanya hanya butuh `UPSTASH_REDIS_URL`
  (`kios-seed` butuh `KIOS_SEED_DIR` juga).

---

## 1. Tiga level pengecekan

| Level | Tujuan | Env yang dibutuhkan |
|-------|--------|---------------------|
| **1. Statis** (build/test/lint) | Pastikan benar sebelum push, tangkap bug | **NOL env** — unit test pakai miniredis (mock) |
| **2. Debug lokal** | Jalankan & coba alur dengan data | Minimal `UPSTASH_REDIS_URL` |
| **3. Smoke test runtime** | Bot + dashboard betulan dengan data | Set lengkap (lihat §3–§5) |

**Perintah Level 1 (tanpa env apa pun):**

```bash
# Bot (Go)
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
make check          # = deps + fmt + vet + test + lint-docs

# Dashboard (Next.js)
cd kios-dashboard
npm run typecheck && npm run lint && npm run build && npm run test
```

> Inilah cara paling efektif menangkap bug sebelum push — tanpa mengisi env.

---

## 2. File `.env.example` (root) — gaya dotenv untuk di-export ke shell

Biarkan baris LLM/channel upstream tetap **dikomentari**. Bagian Kios yang
relevan:

| Variabel | Wajib? | Isi (debug/runtime) | Catatan |
|----------|:------:|---------------------|---------|
| `TZ` | Disarankan | `Asia/Makassar` | Ganti dari `Asia/Shanghai` agar jam laporan/notif (WITA) benar |
| `GROQ_API_KEY` | Ya | `gsk_xxx` (asli) | LLM utama |
| `GEMINI_API_KEY` | Opsional | _(kosong)_ | Fallback |
| `UPSTASH_REDIS_URL` | **WAJIB** | `rediss://default:PASS@db-DEV.upstash.io:6379` | Tanpa ini fitur kios mati |
| `KIOS_REPORT_CHAT` | Opsional | ID Telegram / kosong | Kosong = laporan harian mati |
| `KIOS_REPORT_CRON` | Opsional | _(kosong)_ | Kosong = default `0 18 * * *` |
| `KIOS_DASHBOARD_URL` | Opsional | `http://localhost:3000` | Untuk `/web`, `/login`, tarik summary |
| `KIOS_SERVICE_SECRET` | Opsional | rahasia | Harus sama dengan dashboard |
| `KIOS_PENDING_ALERT_THRESHOLD` | Opsional | _(kosong)_ | Kosong = default `5` |

Muat ke shell (karena bot tak baca `.env` sendiri):

```bash
set -a; . ./.env; set +a
```

> Catatan: file root ini **tidak** memuat `TELEGRAM_BOT_TOKEN` /
> `KIOS_ALLOW_FROM` di bagian kios. Untuk menjalankan bot, `deploy/env.example`
> lebih lengkap (lihat §3).

---

## 3. File `deploy/env.example` — ACUAN UTAMA bot runtime

File ini men-drive bot betulan lewat `deploy/entrypoint.sh`. Empat variabel ini
divalidasi (**FATAL** bila kosong): `TELEGRAM_BOT_TOKEN`, `GROQ_API_KEY`,
`UPSTASH_REDIS_URL`, `KIOS_ALLOW_FROM`.

### Wajib

| Variabel | Contoh | Keterangan |
|----------|--------|-----------|
| `TELEGRAM_BOT_TOKEN` | `123456:ABC-token-bot-UJI` | **Pakai bot terpisah**, bukan produksi |
| `KIOS_ALLOW_FROM` | `111111111` | Telegram ID kamu (dari `@userinfobot`) |
| `GROQ_API_KEY` | `gsk_xxx_asli` | LLM utama; harus valid agar AI menjawab |
| `UPSTASH_REDIS_URL` | `rediss://default:PASS@db-DEV.upstash.io:6379` | **DB dev**, bukan produksi |

### Opsional (untuk menguji fitur tertentu)

| Variabel | Isi | Keterangan |
|----------|-----|-----------|
| `GEMINI_API_KEY` | _(opsional)_ | Aktifkan fallback/routing |
| `ANTHROPIC_API_KEY` | _(opsional)_ | Tier medium / Claude |
| `GROQ_MODEL` | `meta-llama/llama-4-scout-17b-16e-instruct` | Default aman |
| `GEMINI_MODEL` | `gemini-2.0-flash` | Default |
| `ANTHROPIC_MODEL` | `claude-sonnet-4-6` | Default |
| `KIOS_DEFAULT_ROLE` | `owner` | `owner` = akses penuh semua tool |
| `KIOS_REPORT_CHAT` | ID / kosong | Isi untuk melihat laporan harian |
| `KIOS_REPORT_CRON` | `0 18 * * *` | Ubah dekat jam kini untuk uji cepat |
| `KIOS_DASHBOARD_URL` | `http://localhost:3000` | Aktifkan integrasi web |
| `KIOS_SERVICE_SECRET` | `rahasia-min16char` | Harus sama dengan dashboard (uji `/api/summary`) |
| `KIOS_SEED_DIR` | _(kosong saat run bot)_ | Hanya untuk CLI `kios-seed` |
| `PORT` | _(jangan diisi)_ | Default `18790` |

> **Catatan routing (entrypoint):** routing 3-tier aktif bila Gemini **dan**
> Anthropic sama-sama ada. Bila hanya Groq, semua trafik ke Groq — tetap jalan,
> cukup untuk smoke test dasar.
>
> **Var tambahan yang dibaca kode** (opsional, set di shell bila perlu):
> `KIOS_BACKUP_CHAT`, `KIOS_BACKUP_CRON` (default `0 22 * * *`),
> `KIOS_BACKUP_DIR`, `KIOS_TEMPLATE_DIR` (default `templates`), `TZ`.

### Menjalankan smoke test bot (dari root repo)

```bash
set -a; . ./deploy/env.lokal; set +a          # file env kamu — JANGAN commit
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
make build
PICOCLAW_HOME=$PWD/.picoclaw-dev sh deploy/entrypoint.sh   # render config.json + start gateway

# di terminal lain:
curl localhost:18790/health
```

Lalu chat ke bot uji dari akun Telegram yang ID-nya ada di `KIOS_ALLOW_FROM`,
coba `/stok`, `/jual`, `/laporan`, dst.

---

## 4. File `kios-dashboard/.env.example` — salin ke `.env.local`

Hanya perlu jika menguji storefront/login/pesanan, atau integrasi bot `/web`,
`/login`, `/api/summary`.

| Variabel | Isi | Keterangan |
|----------|-----|-----------|
| `UPSTASH_REDIS_REST_URL` | `https://db-DEV.upstash.io` | DB Upstash **SAMA** dengan bot, tapi kredensial **REST** |
| `UPSTASH_REDIS_REST_TOKEN` | `AX...token-rest...` | Tab **REST API** di Upstash Console |
| `TELEGRAM_BOT_TOKEN` | `123456:ABC-token-bot-UJI` | Sama dengan bot uji; **jangan** diawali `bot` |
| `NEXT_PUBLIC_TELEGRAM_BOT_USERNAME` | `namabot_uji_bot` | Tanpa `@` |
| `KIOS_OWNER_IDS` | `111111111` | ID kamu → owner penuh |
| `SESSION_SECRET` | `acak-min32char` | `openssl rand -base64 48` |
| `KIOS_SERVICE_SECRET` | `rahasia-min16char` | Harus sama **persis** dengan bot |

```bash
cd kios-dashboard
cp .env.example .env.local      # lalu isi nilai di atas
npm install
npm run dev                     # http://localhost:3000
```

> Login Telegram lokal: daftarkan domain dashboard di `@BotFather` via
> `/setdomain` (untuk `localhost` umumnya pakai tunnel). Alternatif uji = jalur
> kode `/login` dari bot.

---

## 5. Mengisi data nyata ke Redis

Database dev awalnya kosong. Isi dengan salah satu (keduanya hanya butuh
`UPSTASH_REDIS_URL` yang sama):

```bash
export UPSTASH_REDIS_URL='rediss://default:PASS@db-DEV.upstash.io:6379'

# A) Import dari Excel/CSV (pakai template di templates/)
go run ./cmd/kios-import produk daftar-produk.xlsx
go run ./cmd/kios-import supplier daftar-supplier.csv

# B) Migrasi data CSV lama (butuh KIOS_SEED_DIR; KIOS_SEED_FORCE=1 untuk reset)
export KIOS_SEED_DIR=/path/ke/folder-csv-lama   # berisi stok.csv, transaksi.csv, dst.
go run ./cmd/kios-seed
```

---

## 6. Checklist konsistensi (sumber bug paling sering)

| Harus sama di bot & dashboard | Nilai / cara |
|-------------------------------|--------------|
| Database Upstash | bot `UPSTASH_REDIS_URL` (rediss) ⇄ dashboard `UPSTASH_REDIS_REST_URL` + `_REST_TOKEN` → **DB yang sama** |
| `TELEGRAM_BOT_TOKEN` | identik |
| `KIOS_SERVICE_SECRET` | identik (bila uji `/api/summary`) |
| Owner/whitelist | ID kamu ada di bot `KIOS_ALLOW_FROM` **dan** dashboard `KIOS_OWNER_IDS` |
| `KIOS_DASHBOARD_URL` (bot) | tunjuk ke URL dashboard (`http://localhost:3000`) |

> **Anjuran keselamatan:** gunakan **bot Telegram terpisah** + **database
> Upstash dev terpisah**, jangan kredensial produksi. `kios-seed` punya
> `KIOS_SEED_FORCE=1` yang **me-reset** data.

---

## 7. Ringkasan alur cepat

```text
Sebelum push           →  make check  +  npm run typecheck/build/test   (NOL env)
Debug logika tool      →  go test ./pkg/tools/kios/...                  (NOL env, miniredis)
Jalankan bot betulan   →  isi 4 wajib (deploy/env.example) → entrypoint.sh
Jalankan dashboard     →  isi .env.local → npm run dev
Isi data nyata         →  kios-import / kios-seed (UPSTASH_REDIS_URL)
```
