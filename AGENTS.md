# AGENT.md

Pedoman agen AI (coding assistant maupun agen runtime) untuk repositori
**kios-picoclaw**. File ini mengikuti konvensi [agent.md](https://agent.md/) dan
dapat dipakai oleh berbagai agen pengembang (Claude Code, Cursor, dll.).

---

## 1. Ringkasan Project

**kios-picoclaw** — asisten kios desa berbasis AI, dibangun di atas
[PicoClaw](https://github.com/sipeed/picoclaw) (Go). Tujuannya membantu
operasional kios/warung desa: menjawab pelanggan, info produk/harga, pengingat,
dan tugas ringan lain, lewat aplikasi chat.

| Aspek            | Detail                                                        |
| ---------------- | ------------------------------------------------------------- |
| Bahasa inti      | Go 1.25+                                                       |
| Web UI           | TypeScript + CSS (`web/`)                                      |
| Channel          | Telegram (long polling)                                       |
| Provider LLM     | Groq, Google Gemini                                           |
| Memori/state     | Upstash Redis                                                 |
| Deploy           | Railway (`railway.json`, `Dockerfile`, `DEPLOY-RAILWAY.md`)   |
| Website project  | https://kiosombokpintar04.vercel.app/                         |
| Upstream/landing | https://picoclaw.io/                                          |

---

## 2. Setup & Perintah

```bash
# Dependency
make deps
cd web/frontend && pnpm install --frozen-lockfile   # Node 22+, pnpm 10.33.0+

# Build
make build            # binary inti
make build-launcher   # Web UI launcher
make build-all        # semua platform

# Jalankan
./picoclaw onboard
./picoclaw agent -m "..."   # one-shot
./picoclaw agent            # interaktif
./picoclaw gateway          # gateway (Telegram dll.)

# Kualitas
golangci-lint run     # config: .golangci.yaml
go test ./...
```

---

## 3. Struktur Repo

```
cmd/          Entry point binary
pkg/          Inti Go: agent loop, gateway, channels, providers, tools
config/       Template config (config.example.json)
web/          Web UI Launcher (TS frontend + backend)
docker/       docker-compose & data container
deploy/       Skrip/manifest deploy
docs/         channels/ guides/ reference/ architecture/ security/
integration/  Test integrasi
scripts/      Skrip shell
workspace/    Runtime agent (SKILL.md, memori)
```

---

## 4. Konvensi Kode

- **Go:** idiomatik, lolos `golangci-lint run`. Tambah/ubah test bila mengubah
  perilaku; jalankan `go test ./...`.
- **TypeScript (web/):** ikuti konfigurasi lin/format yang sudah ada di `web/`.
- **Perubahan minimal & selaras pola upstream.** Banyak kode berasal dari
  PicoClaw; jangan refactor besar tanpa alasan kuat.
- **Bahasa pengguna akhir:** Indonesia (ramah, jelas) untuk prompt & pesan kios.
- **Commit:** pesan ringkas dan deskriptif.

---

## 5. Konfigurasi & Rahasia

- Config utama `config.json` (versi 1+). **API key & token TIDAK boleh** di
  `config.json` — taruh di `.security.yml` / environment variable.
- Referensi: `config/config.example.json`, `.env.example`,
  `docs/security/security_configuration.md`.
- Env penting: `PICOCLAW_GATEWAY_HOST` (set `0.0.0.0` untuk Docker/Railway),
  `PICOCLAW_LOG_LEVEL`.

**Rahasia yang dipakai project ini:**
- Groq API key
- Gemini API key
- Telegram bot token
- Upstash Redis (URL + token)

---

## 6. Deployment (Railway)

1. Connect repo ke Railway; build mengikuti `railway.json` + `Dockerfile`.
2. Set environment variable: key Groq/Gemini, Telegram bot token, kredensial
   Upstash Redis, dan `PICOCLAW_GATEWAY_HOST=0.0.0.0`.
3. Service utama: `picoclaw gateway`.
4. Detail langkah: `DEPLOY-RAILWAY.md`.

---

## 7. Aturan Keamanan (WAJIB)

1. **Jangan pernah commit kredensial.** Tidak ada API key/token/Redis secret di
   file yang di-commit. Gunakan `.security.yml` / env var.
2. **Jangan log data sensitif.** Hindari mencetak token/isi pesan pribadi ke log.
3. **Hormati sandbox tool.** Tool eksekusi (file, exec, cron, MCP) punya kebijakan
   keamanan — patuhi pengaturan di `docs/reference/tools_configuration.md`.
4. **Pra-v1.0.** PicoClaw upstream masih berkembang cepat; jangan asumsikan API
   stabil dan jangan deploy ke production tanpa pengujian.

---

## 8. Yang Boleh & Tidak Boleh Diubah Otomatis

**Boleh tanpa konfirmasi:** menambah/memperbaiki test, perbaikan bug lokal,
dokumentasi, penyesuaian prompt Bahasa Indonesia, perbaikan lint.

**Konfirmasi dulu:** perubahan pada agent loop inti, gateway, format config,
skema deployment, atau apa pun yang menyentuh kredensial/keamanan.

---

## 9. Saat Ragu

Cek sumber kebenaran berikut sebelum menebak: `Makefile`, `railway.json`,
`config/config.example.json`, dan `docs/`. Jangan mengarang perintah, opsi
config, atau perilaku tool.