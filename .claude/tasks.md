# TASKS — kios-picoclaw

> Diselaraskan dengan kondisi kode pada 2026-05-30 (branch `main`, rilis v0.2.x).
> Roadmap fitur ke depan ada di `ROADMAP-KIOS.md` — dokumen ini hanya melacak
> selisih antara rencana awal dan kode yang sudah jalan.

---

## ✅ SELESAI (sudah jalan di v0.2.x)

### Fondasi & konektivitas
- [x] `make build` jalan, binary hidup (Dockerfile multi-stage Go→alpine)
- [x] `.env.example` lengkap (TELEGRAM_BOT_TOKEN, GROQ_API_KEY, GEMINI_API_KEY,
      UPSTASH_REDIS_URL, KIOS_ALLOW_FROM, dll.)
- [x] `railway.json` + `deploy/entrypoint.sh` (render config.json dari env)
- [x] Channel Telegram (long polling) + model_list Groq primary / Gemini fallback
- [x] System prompt Bahasa Indonesia, persona asisten kios (`workspace/AGENT.md`, `SOUL.md`)
- [x] Upstash Redis sebagai single source of truth (`store.go`, `store_more.go`)

### Logika kios — diimplementasi sebagai Go tools native (BUKAN SKILL.md saja, lihat D4)
12 tool ter-register via `AllTools(store)` di `register.go`:
- [x] `kios_stok`, `kios_kasir`, `kios_laporan`, `kios_harga` (4 tool inti spec)
- [x] `kios_supplier`, `kios_promo`, `kios_pustaka`, `kios_pasar`, `kios_belajar`
- [x] `kios_user` (RBAC owner/kasir), `kios_import_upload`, `kios_restore`
- [x] Slash commands tanpa LLM (`/stok`, `/jual`, `/laporan`, dll. — `commands.go`)
- [x] `workspace/skills/kios-koperasi/SKILL.md` (persona + aturan pakai tool)

### Otomasi
- [x] Notifikasi stok menipis + pesanan baru (loop 2 menit — `notif.go`)
- [x] Laporan harian otomatis via cron ke Telegram (`report.go`)
- [x] Backup/restore JSON semua data (`backup.go`, `restore.go`)
- [x] Import massal Excel/CSV (Telegram upload + CLI `kios-import`)
- [x] Seed/migrasi data CSV lama (`seed.go`, `cmd/kios-seed`)

### Dashboard Web (Next.js — TIDAK ada di tasks/spec awal, tapi sudah dibuat penuh)
- [x] Admin: dashboard, produk, kasir, laporan, penjualan, suplier, pesanan,
      pengguna, impor, pengaturan
- [x] Storefront publik `/toko` (QRIS + konfirmasi WhatsApp) + `/mall`
- [x] Login Telegram + kode, API routes (auth, health, summary, pesanan, mall)

---

## ⚠️ BELUM DIKERJAKAN — gap penting dari rencana awal

### Manajemen kuota LLM khusus kios (decisions.md D7/D8) — SEBAGIAN BESAR BELUM ADA
Fallback antar-provider (D7 lapis 4) sudah ditangani picoclaw core
(`pkg/providers/error_classifier.go`). Lapisan khusus kios berikut **belum** ada:
- [ ] Cache FAQ di Redis (`kios:cache:faq:{hash}`) — cek cache sebelum panggil LLM
- [ ] Rate limit per-user + debounce pesan beruntun (cegah 1 user habiskan kuota)
- [ ] Counter pemakaian harian (`kios:llm:usage:{tgl}`) per provider
- [ ] Ambang "mode hemat" otomatis (rule-only + cache) saat mendekati limit
- [ ] Notif ke owner saat kuota mendekati / mencapai limit
- [ ] Pesan ramah saat SEMUA provider habis (verifikasi: bukan error mentah ke pembeli)
- [ ] Tes beban: banyak pesan barengan → degradasi mulus, tidak crash

> Keputusan: tuntaskan gap ini, ATAU lanjut ke roadmap fitur. Perlu konfirmasi prioritas.

### Hardening yang belum diverifikasi
- [ ] Audit limit aktual tiap provider saat setup (RPM/RPD Groq, kuota harian Gemini)
- [ ] Set `gateway.log_level=warn`, cek resource usage di Railway
- [ ] Tes end-to-end produksi via Telegram (deploy nyata)

---

## 📋 Roadmap fitur berikutnya
Lihat `ROADMAP-KIOS.md` — Milestone 1 (F1 Hutang, F2 keranjang multi-item,
F3 stok opname, F4 UI promo/pasar, F5 ekspor Excel) sampai Milestone 3.
Catatan: fitur **hutang/kredit** (F1) sudah disebut sejak rencana awal tapi
sampai sekarang belum dibuat.

---

## Catatan teknis (tetap berlaku)
- Jangan commit secret. Kios tools bersifat additive — jangan break picoclaw core.
- Prioritaskan handler rule-based (slash command) sebelum panggil LLM.
- Jangan ubah field name struct tanpa update Go + `kios-dashboard/src/lib/types.ts` sekaligus.
