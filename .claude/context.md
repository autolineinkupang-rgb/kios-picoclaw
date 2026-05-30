# PROJECT CONTEXT — kios-picoclaw

> **Status (per 2026-05-30, v0.2.x):** dokumen ini menangkap VISI & alasan awal.
> Banyak yang sudah jalan dan dilampaui — kini ada **bot Telegram (12 Go tools)**
> PLUS **dashboard Next.js** (admin + storefront `/toko`), bukan cuma bot.
> Untuk arsitektur kode aktual lihat `CLAUDE.md`; selisih rencana↔kode di `tasks.md`;
> status tiap keputusan di `decisions.md`; rencana fitur di `ROADMAP-KIOS.md`.

## Apa ini
Asisten AI untuk kios/warung desa, dibangun di atas PicoClaw (Go, ultra-ringan <10MB).
User (pemilik kios + pelanggan) berinteraksi via Telegram. Otak AI pakai Groq
(cepat/murah) dengan fallback Gemini. State & data disimpan di Upstash Redis.
Deploy ke Railway sebagai single binary dalam container.
(Sejak rencana awal: ditambah **dashboard web Next.js** deploy di Vercel.)

## Stack
- Bahasa: Go 1.25+ (89% codebase)
- Base: fork sipeed/picoclaw — JANGAN rombak arsitektur core, extend lewat channel/tools/skills
- Channel utama: Telegram (long polling, paling mudah, tanpa webhook publik)
- LLM: Groq (primary, model routing untuk query simple) + Gemini (fallback/vision)
- State/DB: Upstash Redis (serverless, REST API — cocok untuk Railway)
- Deploy: Railway (railway.json + Dockerfile sudah ada)

## Domain: Kios Desa
Fungsi yang diharapkan:
- Cek harga & stok barang (data di Redis) — ✅ ada (`kios_stok`, `kios_harga`)
- Catat penjualan (buku warung digital) — ✅ ada (`kios_kasir`, `kios_laporan`)
- Catat utang/kredit pelanggan — ❌ BELUM dibuat (ROADMAP-KIOS.md F1)
- Reminder restock — ✅ ada (notif stok menipis); tagih utang ❌ belum
- Tanya-jawab umum pelanggan (jam buka, produk tersedia) — ✅ via LLM/SKILL
- Bahasa: Indonesia (+ kemungkinan bahasa daerah Kupang/NTT)

## Batasan penting
- Hemat token: query sederhana (cek harga, salam) JANGAN ke LLM, pakai rule/handler dulu
- Hemat resource: target tetap ringan, jangan tambah dependency berat
- Belum production-ready (picoclaw < v1.0) — hati-hati soal security, jangan commit secret
- Secret (API key) masuk .security.yml / env Railway, BUKAN config.json

## Masalah Kuota LLM (KRITIS untuk kios desa)
> **Status: ⚠️ SEBAGIAN ditangani.** Fallback antar-provider + rule-based handler
> sudah jalan; cache FAQ, rate-limit per-user, counter pemakaian harian, dan mode
> hemat BELUM dibuat. Detail per lapisan ada di `decisions.md` D7/D8. Ini gap terbuka.

Semua free tier punya limit ketat. Asisten bisa "mati respon" saat kuota habis,
padahal pelanggan tetap chat. Harus ada strategi berlapis, bukan andalkan 1 provider.

Limit free tier (perkiraan, cek ulang saat setup — bisa berubah):
- Groq: limit per-menit (RPM) & per-hari (RPD) ketat, sering kena "rate limit exceeded"
- Gemini: kuota harian per model + limit per-menit
- Risiko: jam ramai kios = banyak pesan barengan -> langsung kena RPM limit

Prinsip: SEBANYAK MUNGKIN pesan diselesaikan TANPA LLM.
