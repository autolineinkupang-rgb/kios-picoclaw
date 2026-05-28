# Deploy kios-picoclaw ke Railway

Bot kios berbasis picoclaw (Go) — ringan (<50MB RAM), data tersimpan di Upstash Redis,
dioperasikan lewat Telegram. Panduan ini memakai **Dockerfile** di root repo.

> Catatan tier: Railway tidak punya free tier permanen yang besar. Ada **Trial** ($5 kredit
> sekali pakai, 30 hari) dan **Free plan** ($0/bln, ~$1 kredit/bln, 0.5 GB RAM, 1 project).
> Footprint picoclaw kecil sehingga muat di RAM Free plan; batasnya ada di kredit usage untuk
> layanan 24/7. Untuk pemakaian stabil, plan **Hobby** ($5/bln) paling aman.

## 1. Siapkan Upstash Redis (gratis)
1. Buat akun di https://console.upstash.com → **Create Database** (Redis, region terdekat mis. Singapore).
2. Salin **`UPSTASH_REDIS_URL`** bentuk `rediss://default:<password>@<host>.upstash.io:6379`
   (pakai TLS endpoint / "rediss"). Data kios bertahan walau container redeploy.

## 2. Siapkan bot Telegram
1. Chat **@BotFather** → `/newbot` → simpan **`TELEGRAM_BOT_TOKEN`**.
2. Chat **@userinfobot** untuk dapat **user ID** kamu (angka). Kumpulkan semua id yang boleh
   pakai bot, pisah koma → **`KIOS_ALLOW_FROM`** (mis. `111111111,222222222`).
3. (opsional) Di BotFather set `/setprivacy` → **Disable** kalau bot dipakai di grup.

## 3. Siapkan Groq (+ Gemini / Claude opsional)
- Groq API key dari https://console.groq.com/keys → **`GROQ_API_KEY`** (LLM utama, gratis/cepat).
- (opsional) Gemini API key dari https://aistudio.google.com/app/apikey → **`GEMINI_API_KEY`** (cadangan).
- (opsional) Anthropic API key dari https://console.anthropic.com/settings/keys → **`ANTHROPIC_API_KEY`** (cadangan Claude, model default `claude-sonnet-4-6`).

## 4. Deploy ke Railway
1. Push repo ini ke GitHub.
2. Railway → **New Project → Deploy from GitHub repo** → pilih repo ini.
   Railway mendeteksi `railway.json` + `Dockerfile` otomatis.
3. Buka tab **Variables**, isi (lihat `.env.example`):
   - `TELEGRAM_BOT_TOKEN`
   - `KIOS_ALLOW_FROM`
   - `GROQ_API_KEY`
   - `UPSTASH_REDIS_URL`
   - (opsional) `GEMINI_API_KEY`, `ANTHROPIC_API_KEY`, `KIOS_DEFAULT_ROLE`, `GROQ_MODEL`, `GEMINI_MODEL`, `ANTHROPIC_MODEL`
   - **Laporan & Alert bot** (opsional, aktif bila diisi):
     - `KIOS_REPORT_CHAT` — chat ID Telegram tujuan laporan harian otomatis
     - `KIOS_REPORT_CRON` — jadwal cron (default `0 18 * * *`, TZ Asia/Makassar)
     - `KIOS_DASHBOARD_URL` — base URL dashboard (mis. `https://kios-dashboard.vercel.app`)
     - `KIOS_SERVICE_SECRET` — rahasia HMAC; **harus sama** dengan `KIOS_SERVICE_SECRET` di dashboard
     - `KIOS_PENDING_ALERT_THRESHOLD` — ambang pesanan pending yang memicu alert (default `5`)
   > `PORT` di-set otomatis oleh Railway — jangan diisi manual.
4. **Deploy**. Railway build dari Dockerfile (≈2–4 menit). Entrypoint merender `config.json`
   dari Variables, lalu menjalankan `picoclaw gateway`.

## 5. Verifikasi
- Log Railway harus menampilkan `kios-picoclaw: starting gateway on 0.0.0.0:$PORT`
  lalu `Gateway started`. Healthcheck `/health` jadi hijau.
- Chat bot di Telegram (dari user id yang ada di `KIOS_ALLOW_FROM`):
  - "cek stok" → daftar produk
  - "jual 2 beras bayar 50000" → struk + kembalian
  - "laporan hari ini" → ringkasan + laba
- User di luar whitelist tidak akan direspons (gate `allow_from`).

## 6. (Opsional) Migrasi data lama → jalankan SENDIRI di lokal
Data baru mulai kosong (Redis). Impor stok/transaksi/harga lama pakai seeder bawaan.
**Jalankan di komputermu sendiri** supaya URL Upstash (rahasia) tidak bocor ke repo/chat:

```bash
cd ~/kios-picoclaw
UPSTASH_REDIS_URL='rediss://...PUNYAMU...' \
KIOS_SEED_DIR=/home/kevinman/kios-openclaw/data \
~/sdk/go/bin/go run ./cmd/kios-seed
```

- Mengimpor `stok.csv, transaksi.csv, pembelian.csv, price-history.csv` (idempotent — sekali jalan;
  pakai `KIOS_SEED_FORCE=1` untuk paksa ulang).
- **`users.json` TIDAK diimpor** (ber-key nomor HP era Signal, tak cocok dengan ID Telegram, lagipula PII).
  Daftarkan pengguna lewat tool `kios_user` (owner: "tambah kasir <id telegram> nama <nama>").
- Setelah selesai, cek di Telegram: kirim `/stok` → produk lama muncul.

## Isi data massal lewat form Excel (.xlsx) atau CSV
Mau isi banyak produk/supplier sekaligus? Pakai template di folder `templates/`:
- **`templates/produk-template.xlsx`** & **`templates/supplier-template.xlsx`** — file Excel asli (header tebal, contoh baris)
- atau versi **`.csv`** kalau lebih suka teks polos

Langkah:
1. Buka template `.xlsx` di **Excel / Google Sheets**, isi barisnya (hapus contoh).
2. Import langsung ke Redis (jalankan di komputermu, URL tetap lokal) — bisa `.xlsx` ATAU `.csv`:
   ```bash
   cd ~/kios-picoclaw
   UPSTASH_REDIS_URL='rediss://...PUNYAMU...' ~/sdk/go/bin/go run ./cmd/kios-import produk daftar-produk.xlsx
   UPSTASH_REDIS_URL='rediss://...PUNYAMU...' ~/sdk/go/bin/go run ./cmd/kios-import supplier daftar-supplier.xlsx
   ```
   Baris dicocokkan dengan **nama**: yang sudah ada di-update, yang baru dibuat (id otomatis).
   Kolom kosong tidak menimpa nilai lama. Output: "Dibuat: X | Diupdate: Y | Dilewati: Z".

> Regenerasi template Excel: `~/sdk/go/bin/go run ./cmd/gen-templates`. Library Excel (excelize)
> hanya dipakai tool lokal ini, TIDAK masuk ke binary bot.

## Catatan RBAC
- `allow_from` = gerbang utama (hanya id terdaftar yang bisa pakai bot).
- Peran (`kasir`/`owner`) diambil dari Redis `kios:users` berdasarkan Telegram ID pengirim.
  Selama belum diisi, semua user whitelist dianggap `KIOS_DEFAULT_ROLE` (default `owner`).
  Set `KIOS_DEFAULT_ROLE=kasir` untuk mengunci aksi destruktif sampai owner didaftarkan.
