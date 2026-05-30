# DECISIONS — kios-picoclaw

> Decision log bersifat historis (append + anotasi, tidak dihapus).
> Baris **Status** ditambahkan 2026-05-30 untuk menyelaraskan dengan kode.

## D1. Pakai Telegram long-polling, bukan webhook
Alasan: Railway tidak selalu kasih URL publik stabil untuk hobby tier; long-polling
tidak butuh inbound URL. Trade-off: 1 koneksi persisten (oke untuk skala kios).
**Status: ✅ BERLAKU & TERIMPLEMENTASI** — channel Telegram native picoclaw.

## D2. Groq primary + Gemini fallback via model_list
Alasan: Groq inference cepat & murah -> cocok untuk respon pelanggan realtime.
Gemini dipakai untuk vision (foto barang/struk) dan saat Groq rate-limit.
Implementasi: smart routing picoclaw (simple -> Groq kecil, kompleks/vision -> Gemini).
**Status: ✅ BERLAKU** — model_list di config; fallback antar-provider ditangani
picoclaw core (`pkg/providers/error_classifier.go`).

## D3. Upstash Redis sebagai single source of truth
Alasan: serverless, REST-friendly, free tier cukup untuk 1 kios, persist antar restart
container Railway (filesystem Railway ephemeral). Skema data (rancangan awal):
- kios:produk:{sku}     -> {nama, harga, stok}
- kios:utang:{nama}     -> {total, riwayat[]}
- kios:penjualan:{tgl}  -> list transaksi
- kios:config           -> jam buka, nama kios

**Status: ✅ BERLAKU, ⚠️ SKEMA BERKEMBANG.** Skema final lebih kaya dan berbeda
dari rancangan di atas — lihat tabel lengkap di `CLAUDE.md` ("Redis Data Model").
Ringkasnya: `kios:produk` HASH, `kios:transaksi`/`kios:pembelian`/`kios:price_history`
LIST, `kios:users`/`kios:supplier`/`kios:promo`/`kios:config` HASH, plus counter
`kios:seq:*`. Key `kios:utang` BELUM ada (fitur hutang belum dibuat — lihat ROADMAP F1).

## D4. Logika kios dibangun sebagai Skill, bukan fork core
Alasan: jaga upstream picoclaw tetap bisa di-merge. Buat SKILL.md di workspace
untuk perintah kios (cek-harga, catat-jual, catat-utang, restock).

**Status: 🔄 DISESUAIKAN (lihat KIOS_BUILD_SPEC.md).** Logika kios akhirnya dibuat
sebagai **Go tools native** di `pkg/tools/kios/` (12 tool ter-register via
`AllTools(store)`), BUKAN hanya SKILL.md. Prinsip "additive, jangan rombak core"
tetap dijaga: tools cuma di-inject di `agent_init.go`. SKILL.md tetap ada
(`workspace/skills/kios-koperasi/`) untuk persona + aturan kapan memanggil tool.

## D5. Secrets via Railway env + .security.yml
Alasan: keamanan + picoclaw v1 memang pisahkan sensitive ke .security.yml.
JANGAN pernah commit token Telegram/Groq/Gemini/Upstash.
**Status: ✅ BERLAKU & TERIMPLEMENTASI** — env Railway + `deploy/entrypoint.sh`
render config.json dari env; `.env.example` mendokumentasikan semua var.

## D6. Bahasa default Indonesia
System prompt agent di-set Bahasa Indonesia, ramah, ringkas (target warga desa).
**Status: ✅ BERLAKU & TERIMPLEMENTASI** — `workspace/AGENT.md`, `SOUL.md`,
dan SKILL.md kios.

## D7. Strategi anti-kuota berlapis (defense in depth)
Lapisan dari murah -> mahal, LLM adalah pilihan TERAKHIR:

1. Rule/handler dulu (0 token)
   - Salam, /harga, /jual, /utang, jam buka, daftar barang -> ditangani kode, bukan LLM.
   - Target: 70-80% pesan kios tidak pernah menyentuh LLM.

2. Cache jawaban di Redis (0 token untuk hit)
   - Pertanyaan FAQ yang sama (mis. "kios buka jam berapa") -> simpan jawaban,
     key: kios:cache:faq:{hash-pertanyaan}, TTL panjang.

3. Model routing hemat
   - Query pendek/simple -> model Groq paling kecil & cepat.
   - Hanya query kompleks / vision -> Gemini.

4. Fallback berantai antar provider
   - Groq kena rate-limit -> otomatis lempar ke Gemini -> kalau dua-duanya habis,
     balas pesan ramah ("Maaf, asisten lagi sibuk, coba sebentar lagi") --
     JANGAN tampilkan error mentah ke pelanggan.

5. Rate limit & antrian di sisi kita
   - Batasi panggilan LLM per user (mis. max N/menit) -> cegah 1 orang spam habiskan kuota.
   - Debounce: gabungkan pesan beruntun dari user yang sama sebelum panggil LLM.

**Status (per 2026-05-30): ⚠️ SEBAGIAN.**
- Lapis 1 (rule/handler) ✅ — slash commands `/stok`, `/jual`, dll. di `commands.go`.
- Lapis 3 (model routing) ✅ — via model_list.
- Lapis 4 (fallback berantai) ✅ — picoclaw core `pkg/providers/error_classifier.go`.
- Lapis 2 (cache FAQ) ❌ BELUM — tidak ada `kios:cache:faq` di kode.
- Lapis 5 (rate-limit per-user + debounce kios) ❌ BELUM ada di layer kios.
- Pesan ramah saat semua provider habis ❌ belum diverifikasi end-to-end.

## D8. Kuota budget harian eksplisit
Simpan counter pemakaian di Redis (kios:llm:usage:{tgl}) per provider.
Kalau mendekati limit harian -> otomatis turun ke mode hemat (rule-only + cache),
dan kirim notif ke pemilik kios. Tujuan: layanan TIDAK mati total saat kuota habis.

**Status: ❌ BELUM TERIMPLEMENTASI.** Tidak ada `kios:llm:usage`, mode hemat, atau
notif kuota di kode. Ini gap terbuka terbesar dari rencana awal — lihat `tasks.md`
bagian "Manajemen kuota LLM khusus kios".
