# CLAUDE.md

Panduan untuk Claude Code (claude.ai/code) saat bekerja di repositori ini.

## Tentang Project

**kios-picoclaw** adalah asisten kios desa berbasis AI, dibangun di atas
[PicoClaw](https://github.com/sipeed/picoclaw) — asisten AI ultra-ringan yang
ditulis sepenuhnya dengan **Go**. Project ini mengadaptasi PicoClaw untuk kasus
penggunaan kios/warung di desa: menjawab pertanyaan pelanggan, mengelola
informasi produk, dan berinteraksi lewat aplikasi chat.

- **Bahasa inti:** Go (1.25+)
- **Frontend / Web UI:** TypeScript + CSS (di `web/`)
- **Channel utama:** Telegram (long polling)
- **Provider LLM:** Groq dan Google Gemini
- **Penyimpanan / memori:** Upstash Redis
- **Deployment:** Railway (lihat `DEPLOY-RAILWAY.md` dan `railway.json`)
- **Landing page produk:** https://picoclaw.io/ (upstream), website project: https://kiosombokpintar04.vercel.app/

> Karena ini berbasis PicoClaw, sebagian besar arsitektur inti mengikuti upstream.
> Saat ragu soal perilaku inti agent/gateway, periksa dokumentasi di `docs/`
> sebelum mengubah kode.

## Perintah Umum

Project menggunakan `Makefile`. Perintah inti:

```bash
make deps              # Pasang dependency Go
make build             # Build binary inti untuk platform saat ini
make build-launcher    # Build Web UI Launcher (mode WebUI)
make build-all         # Build untuk semua platform di Makefile
make install           # Build lalu install
```

Frontend (Web UI) butuh Node.js 22+ dan pnpm 10.33.0+:

```bash
cd web/frontend && pnpm install --frozen-lockfile
```

### Menjalankan secara lokal

```bash
./picoclaw onboard            # Inisialisasi config & workspace (~/.picoclaw/config.json)
./picoclaw agent -m "Halo"    # Tanya sekali jalan
./picoclaw agent              # Mode chat interaktif
./picoclaw gateway            # Jalankan gateway untuk integrasi chat app (Telegram, dll.)
./picoclaw status             # Cek status
```

### Lint & Test

```bash
golangci-lint run             # Konfigurasi di .golangci.yaml
go test ./...                 # Jalankan seluruh test Go
```

### Docker

```bash
docker compose -f docker/docker-compose.yml --profile launcher up -d
docker compose -f docker/docker-compose.yml logs -f
docker compose -f docker/docker-compose.yml --profile launcher down
```

## Struktur Direktori

```
cmd/          Entry point binary (main package)
pkg/          Library inti Go: agent loop, gateway, channel, provider, tools
config/       Template konfigurasi (lihat config.example.json)
web/          Web UI Launcher (frontend TypeScript + backend)
docker/       Dockerfile compose & data untuk container
deploy/       Skrip & manifest deployment
docs/         Dokumentasi: channels/, guides/, reference/, architecture/, security/
integration/  Test integrasi
scripts/      Skrip bantu (shell)
examples/     Contoh (mis. pico-echo-server)
workspace/    Workspace runtime agent (SKILL.md, memori, dsb.)
```

## Konfigurasi

- Config utama: `config.json` (format versi 1+). Data sensitif (API key)
  dipisah ke `.security.yml` — JANGAN menaruh API key langsung di `config.json`
  pada versi 1+. Lihat `docs/security/security_configuration.md`.
- `config/config.example.json` adalah template lengkap semua opsi.
- `.env.example` mendokumentasikan environment variable yang relevan.
- Variabel lingkungan penting:
  - `PICOCLAW_GATEWAY_HOST` — set `0.0.0.0` untuk Docker/Railway agar dapat diakses.
  - `PICOCLAW_LOG_LEVEL` — `debug|info|warn|error|fatal` (default `warn`).

### Provider (project ini)

Format `protocol/model` di `model_list`:

- Groq: `groq/...` (mis. Llama, Mixtral) — butuh API key.
- Gemini: `gemini/...` (mis. Gemini Flash/Pro) — butuh API key.

### Channel (project ini)

- Telegram: butuh bot token, memakai long polling. Panduan di
  `docs/channels/telegram/README.md`.

### Memori / Redis

- Upstash Redis dipakai untuk penyimpanan memori/state. Konfigurasikan
  kredensial Upstash lewat `.security.yml` / environment variable, bukan di repo.

## Deployment (Railway)

- Konfigurasi build/deploy ada di `railway.json` dan `Dockerfile`.
- Panduan lengkap: `DEPLOY-RAILWAY.md`.
- Pastikan environment variable berikut diset di Railway:
  - API key Groq dan/atau Gemini
  - Telegram bot token
  - Kredensial Upstash Redis
  - `PICOCLAW_GATEWAY_HOST=0.0.0.0`
- Service utama menjalankan `picoclaw gateway`.

## Aturan untuk Claude

1. **Keamanan kredensial — prioritas utama.** Jangan pernah menulis API key,
   bot token, atau kredensial Redis ke dalam file yang di-commit. Gunakan
   `.security.yml` / environment variable. Jika menemukan kredensial ter-hardcode,
   tandai ke pengguna.
2. **Hormati pemisahan upstream.** Banyak kode berasal dari PicoddClaw upstream.
   Untuk perubahan inti agent/gateway/provider, baca `docs/` lebih dulu dan
   buat perubahan minimal yang sejalan dengan pola yang ada.
3. **Idiomatik Go.** Ikuti gaya yang sudah ada, jalankan `golangci-lint run`
   dan `go test ./...` sebelum menganggap pekerjaan selesai.
4. **Konteks lokal.** Project ini melayani kios desa berbahasa Indonesia —
   prompt, pesan, dan dokumentasi yang dihadapkan ke pengguna akhir sebaiknya
   dalam Bahasa Indonesia yang jelas dan ramah.
5. **Jangan deploy ke production sebelum stabil.** Upstream PicoClaw masih dalam
   pengembangan cepat (pra-v1.0); berhati-hati dengan asumsi soal stabilitas API.
6. **Verifikasi sebelum klaim.** Jika tidak yakin soal perintah build, perilaku
   tool, atau opsi config, cek `Makefile`, `docs/`, dan file config nyata — jangan
   mengarang.