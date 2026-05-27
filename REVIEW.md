# Laporan Review Project: kios-picoclaw

> Dibuat: 2026-05-27 | Metode: Swarm 5 agen paralel, read-only

---

## 1. Identitas Project

| Atribut | Detail |
|---------|--------|
| Module | `github.com/sipeed/picoclaw` |
| Go Version | 1.25.10 |
| Lisensi | MIT |
| Status | v0.2.x — belum production-ready (per README sendiri) |
| Tujuan | Asisten AI kasir warung/toko desa berbahasa Indonesia via Telegram |

---

## 2. Arsitektur

**Pola:** Hybrid DDD + Clean Architecture

**5 Binary:**
- `picoclaw` — agent utama (CLI + daemon)
- `picoclaw-launcher` — web console (Wails)
- `membench` — memory benchmark
- `kios-seed`, `kios-import` — utilitas data

**38 Package** di `pkg/`:

| Layer | Package |
|-------|---------|
| Domain | `agent/`, `channels/` (21 kanal) |
| Application | `gateway/`, `routing/`, `tools/` |
| Infrastructure | `config/`, `credential/`, `migrate/`, `memory/` |
| Interface | Multi-channel handlers, web frontend/backend |

**Dependencies utama:** Anthropic SDK v1.26.0, OpenAI SDK v3.22.0, AWS Bedrock, Redis, Telegram, Discord, WhatsApp, Slack, Matrix, WebRTC, Cobra, Zerolog

---

## 3. Fitur Bisnis

- **Kasir/POS** — penjualan produk, struk, kembalian otomatis, metode pembayaran (tunai/transfer/QRIS)
- **Manajemen Stok** — tracking produk, stok minimum/kritis, pencarian
- **Restocking** — pencatatan pembelian supplier, log perubahan harga otomatis
- **Laporan** — harian/mingguan/bulanan, laba (omzet − modal), produk terlaris
- **Supplier** — daftar supplier, tracking pembelian
- **Promosi** — diskon nominal/persen, minimum quantity, date range
- **Knowledge Base** (`pustaka`) — basis pengetahuan dari data penjualan

**Interface utama:** Telegram Bot (slash commands `/stok`, `/jual`, `/kasir`, `/laporan`) — bukan web dashboard.

---

## 4. Keamanan & RBAC

**Identifikasi:** Berbasis Telegram ID (tidak ada password/JWT).

| Peran | Akses |
|-------|-------|
| `owner` | Penuh — semua operasi termasuk hapus, reset, manajemen user |
| `kasir` | Terbatas — hanya jual dan lihat stok |

- Gating diterapkan di level setiap tool via `resolveRole()` + `requireOwner()`
- Owner permanen dikonfigurasi via `KIOS_OWNER_IDS` env var (bootstrap protection)
- User inactive (`aktif=false`) ditolak otomatis

**Postur keamanan:**

| Risiko | Tingkat | Catatan |
|--------|---------|---------|
| Telegram account compromise = full access | Medium | Tidak ada 2FA tambahan di level app |
| Data di managed Redis (Upstash) | Medium | Credentials di env var Railway |
| Command injection | Low | Ada `urlsafety.go` dengan regex scoring |
| SQL injection | Tidak ada | Redis only, tidak ada SQL |

---

## 5. Deployment & DevOps

**Docker:** Multi-stage build — `golang:1.25-alpine` → `alpine:3.23` (~50MB runtime)

**Platform:** Railway (via `railway.json` + `Dockerfile`)

**4 Environment Variable Wajib:**

```
TELEGRAM_BOT_TOKEN    # dari @BotFather
KIOS_ALLOW_FROM       # whitelist Telegram user ID (comma-separated)
GROQ_API_KEY          # LLM utama (https://console.groq.com)
UPSTASH_REDIS_URL     # persistence Redis (rediss://)
```

**CI/CD:** `.github/workflows/kios.yml` — build + test `pkg/tools/kios/**` pada push/PR

**Potensi masalah deployment:**
- Timezone hardcoded WITA (+8) — perlu sinkronisasi clock di Railway
- Groq + Gemini + Telegram polling bersamaan = potensi bottleneck banyak user
- `integration/suites/` masih kosong
- Health check `/health` harus accessible (verifikasi via Railway logs)

---

## 6. Kualitas Kode

**Baik:**
- Interface-driven design konsisten (12+ interface: `Tool`, `SessionStore`, `LLMProvider`, dll)
- ~3044 error check, custom error types (`ErrBusClosed`, `ErrMissingInboundContext`)
- Context propagation type-safe di seluruh codebase
- Konvensi penamaan Go sesuai standar penuh

**Perlu perbaikan:**
- Duplikasi `ErrorResult()` di `tools/shared_facade.go` dan `tools/hardware/shared.go`
- Godoc tidak konsisten di `gateway/` dan `commands/`
- Mix format error wrapping: `fmt.Errorf("msg: %w", err)` vs `fmt.Errorf("msg")`

---

## 7. Testing

**176 test file | ~129.000 baris test code**

| Area | Status |
|------|--------|
| Shell execution, tool registry, session management | Kuat |
| Channels, providers, memory persistence | Sedang |
| E2E integration, security boundary, performance/stress | Lemah |

**Gap utama:**
- `integration/suites/` kosong — tidak ada E2E test untuk Telegram, Discord, atau provider nyata
- Tidak ada mock standar untuk provider testing
- Tidak ada security boundary testing (SSRF, sandbox isolation)
- Tidak ada load/stress testing

---

## 8. Dokumentasi

| File | Ukuran | Nilai |
|------|--------|-------|
| `README.md` | 29.657 baris | Sangat lengkap — setup 6+ platform, 30+ provider, 19+ channel |
| `PERINTAH.md` | 4.057 baris | Panduan operasional untuk user non-teknis |
| `ROADMAP.md` | 5.634 baris | 7 pilar strategis, prioritas jelas |
| `CONTRIBUTING.md` | Lengkap | Community roles, workflow kontribusi |
| `docs/` | Beberapa panduan | Hooks, steering, subturn, channel setup |

---

## 9. Roadmap (7 Pilar Strategis)

1. **Memory Footprint** — target <20MB RAM di board 64MB
2. **Security Hardening** — prompt injection defense, sandboxing, OAuth 2.0
3. **Protocol-First Connectivity** — refactor arsitektur vendor→protocol
4. **Advanced Capabilities** — browser automation, mobile control
5. **DevEx & Documentation** — CLI wizard interaktif
6. **AI-Powered Engineering** — automated code review, issue triage
7. **Brand & Community** — logo, mantis shrimp branding

**Item prioritas tinggi:** Memory optimization, provider architecture refactor, browser automation, smart model routing dispatch.

---

## 10. Ringkasan Eksekutif

Project `kios-picoclaw` adalah AI agent berbahasa Indonesia untuk kasir warung desa dengan arsitektur yang cukup matang untuk ukuran v0.2.x. Modularitas tinggi (38 package, 21 channel, LLM-agnostic), dokumentasi sangat lengkap, dan unit test solid.

**Kekuatan utama:** Modularitas, dokumentasi, multi-channel support, RBAC tool-level  
**Gap terbesar:** E2E/integration testing kosong, security boundary belum ditest, belum production-ready  
**Rekomendasi segera:** Isi `integration/suites/`, tambah mock provider standar, dan verifikasi health check Railway sebelum promosi ke pengguna lebih luas
