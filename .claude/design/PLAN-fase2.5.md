# PLAN Konsolidasi — Sisa Fase 2 + Fase 2.5

> Hasil gabungan dari 5 desain paralel (agen arsitek ruflo, 2026-05-30).
> Mode: **desain-saja** — belum ada kode produksi. Dokumen ini menyatukan kelimanya,
> merekonsiliasi konflik antar-desain, dan menetapkan urutan implementasi untuk di-review.
>
> Desain rinci per fitur: `01-utang.md`, `02-cache-faq.md`, `03-rate-limit.md`,
> `04-counter-mode-hemat.md`, `05-notif-kuota.md` (folder ini).

---

## 0. Ringkasan eksekutif

Lima fitur, **dua jalur independen**:

- **Jalur A — Utang (`kios_hutang`)**: fitur Fase 2 yang tersisa. Berdiri sendiri,
  tidak menyentuh LLM. Bisa dikerjakan kapan saja, paralel dengan Jalur B.
- **Jalur B — Subsistem Kuota LLM (4 fitur)**: cache FAQ, rate-limit, counter+mode
  hemat, notif kuota. Keempatnya **berbagi satu lapis hook** sebelum/sesudah panggilan
  LLM. Harus dikerjakan sebagai satu subsistem terkoordinasi.

**Temuan kunci (terverifikasi di kode):** picoclaw sudah punya sistem hook LLM aditif
yang persis cocok — `LLMInterceptor{BeforeLLM, AfterLLM}` di `pkg/agent/hooks.go`,
dipanggil `Pipeline.CallLLM` (`pipeline_llm.go:82` & `:503`), di-mount via
`HookManager.Mount(NamedHook(...))` dengan `Priority`. Hook bisa `HookActionAbortTurn`
untuk memintas LLM. **Tidak perlu merombak core picoclaw** — semua fitur kuota
cukup di-mount sebagai hook dari sisi kios di `agent_init.go`.

---

## 1. Jalur A — Utang (`kios_hutang`) — INDEPENDEN

Ringkasan dari `01-utang.md`:

- **Tool** `kios_hutang`, actions: `catat`, `bayar` (auto-lunas bila ≥ sisa), `daftar`,
  `detail`, `lunas` (manual). RBAC: catat/bayar = kasir+owner; `lunas` manual = owner.
- **Struct** `Hutang{id, pelanggan_nama, pelanggan_kontak, items[], total, terbayar,
  sisa, tanggal, kasir, catatan, status, tanggal_lunas, transaksi_id, riwayat_bayar[]}`.
- **Redis**: `kios:hutang` (HASH) + `kios:seq:hut` (→ HUT-0001). Tidak ada env baru.
- **Integrasi**: struct+const → `store.go`; file baru `hutang.go` (<400 baris); 1 baris
  `register.go`; entry `commands.go` (`/hutang [nama]`). Dashboard: `types.ts`,
  `redis.ts`, `kios.ts`, `(app)/hutang/` (page+actions), `components/hutang/`, 1 nav item.
- **Keputusan terbuka (perlu jawaban user)** → lihat §6.

Tidak ada interaksi dengan Jalur B. Aman dikerjakan kapan saja.

---

## 2. Jalur B — Subsistem Kuota LLM (kontrak bersama)

Keempat agen konvergen ke arsitektur hook yang sama. Berikut **kontrak terpadu** hasil
rekonsiliasi (ini bagian "gabungkan" yang sebenarnya):

### 2.1 Lapis hook & urutan eksekusi

Semua hook di-mount ke `HookManager` yang sama di `agent_init.go` (setelah store kios
dibuat). Urutan via `Priority` (angka kecil = duluan):

| Fase | Priority | Hook | Aksi |
|------|:--:|------|------|
| **BeforeLLM** | 10 | Cache FAQ (`KiosFAQHook`) | HIT → publish jawaban + `AbortTurn` (0 token) |
| **BeforeLLM** | 20 | Rate-limit (`KiosRateLimitHook`) | Lewat batas → publish pesan ramah + `AbortTurn` |
| **BeforeLLM** | 30 | Mode hemat (`KiosQuotaHook`) | Di atas ambang → publish pesan ramah + `AbortTurn` |
| → | — | **LLM dipanggil** | hanya jika tidak ada hook yang abort |
| **AfterLLM** | 10 | Cache FAQ write-through | simpan jawaban cacheable |
| **AfterLLM** | 30 | Counter usage (`KiosQuotaHook`) | `INCR` usage provider (hanya bila sukses) |

> **Rekonsiliasi penting:** Cache dicek **sebelum** rate-limit & counter, jadi cache-hit
> tidak menambah counter dan tidak kena rate-limit (0 token = bukan pemakaian LLM).
> Angka priority 10/20/30 sudah disepakati identik oleh keempat agen.

### 2.2 Notif kuota BUKAN hook (koreksi konflik)

Agen #2 sempat berasumsi notif jadi AfterLLM hook (priority 50). **Desain final (#5)
menolak itu**: notif berjalan sebagai **polling di `NotifService.loop()` yang sudah ada**
di `notif.go` (~tiap 2 menit), membaca counter, lalu `sendToOwners()`. Lebih sederhana &
aditif. → Di AfterLLM hanya ada cache-write (10) dan counter (30); notif terpisah.

### 2.3 Namespace Redis bersama (final)

| Key | Tipe | Pemilik | Pembaca |
|-----|------|---------|---------|
| `kios:cache:faq:{sha256_8hex}` | STRING(JSON) | #2 | #2 |
| `kios:cache:faq:version` | STRING(int) | #2 | #2 (invalidasi generasional) |
| `kios:llm:ratelimit:{userID}:min` / `:hour` | STRING(INCR+TTL) | #3 | #3 |
| `kios:llm:usage:{provider}:{YYYY-MM-DD}` | STRING(INCR, EXPIRE 48j) | #4 | #4, #5 |
| `kios:llm:notif_sent:{provider}:{YYYY-MM-DD}:{level}` | STRING(TTL 25j) | #5 | #5 |

- **Provider canonical: `groq`, `gemini`, `claude`** (lowercase, cocok `model_name` di
  `entrypoint.sh`). → mengoreksi asumsi `anthropic` di draf #5.
- **Tanggal: zona Asia/Makassar (UTC+8)**, konsisten dgn laporan harian.
- **Mode hemat dihitung on-the-fly** dari counter vs limit — **tidak ada** key state
  `kios:llm:frugal_mode`/`mode_hemat`. → mengoreksi asumsi flag di draf #5.
- **Limit disimpan di ENV, bukan Redis** (`kios:llm:limit:*` dari draf #5 dibatalkan);
  #5 membaca limit dari env yang sama dengan #4.

### 2.4 Helper bersama yang harus dibuat lebih dulu

- `providerFromModel(modelName) string` → map `groq-llama→groq`, `gemini-flash→gemini`,
  `claude→claude`. Dipakai #4 (counter) & #5 (notif). Saat ini heuristik prefix —
  **finalisasi konvensi di spike S2** (lihat §5).
- Injeksi `*bus.MessageBus` ke tiap hook (pola sama `NotifService`) agar bisa publish
  pesan ramah saat `AbortTurn`. Konstanta key Redis di satu tempat (`store.go`).

---

## 3. Variabel environment baru (konsolidasi)

> Disarankan **menyeragamkan prefix ke `KIOS_LLM_*`** untuk seluruh subsistem kuota
> (draf #2 memakai `KIOS_FAQ_*`). Daftar final usulan:

| Env | Default | Fitur | Keterangan |
|-----|---------|:--:|-----------|
| `KIOS_LLM_CACHE_ENABLED` | `true` | #2 | on/off cache FAQ |
| `KIOS_LLM_CACHE_TTL` | `86400` | #2 | detik |
| `KIOS_LLM_CACHE_MAX_ENTRIES` | `500` | #2 | batas entri |
| `KIOS_LLM_RATELIMIT_ENABLED` | `true` | #3 | on/off |
| `KIOS_LLM_RPM_LIMIT` | `5` | #3 | panggilan/menit/user |
| `KIOS_LLM_RPH_LIMIT` | `30` | #3 | panggilan/jam/user |
| `KIOS_DEBOUNCE_ENABLED` | `false` | #3 | gabung pesan beruntun |
| `KIOS_DEBOUNCE_MS` | `1500` | #3 | jendela debounce |
| `KIOS_LLM_QUOTA_ENABLED` | `false` | #4 | master switch counter+hemat |
| `KIOS_LLM_LIMIT_GROQ/GEMINI/CLAUDE` | `0` | #4,#5 | 0 = disabled |
| `KIOS_LLM_HEMAT_PCT_GROQ/GEMINI/CLAUDE` | `80` | #4 | % ambang mode hemat |
| `KIOS_LLM_NOTIF_ENABLED` | `false` | #5 | on/off notif owner |
| `KIOS_LLM_NOTIF_THRESHOLDS` | `80,100` | #5 | persen pemicu |

Owner notif memakai `KIOS_OWNER_IDS` (sudah ada). Semua default konservatif: subsistem
kuota **mati** sampai limit aktual diaudit (lihat S? di §5) lalu diaktifkan bertahap.

---

## 4. Urutan implementasi & dependensi

```
Jalur A (Utang)  ──────────────────────────────────►  [bisa kapan saja, paralel]

Jalur B (Kuota LLM):
  B0. Fondasi bersama  ─►  B1 Cache  ─┐
      (helper provider,    B2 Rate-limit ─┼─► B3 Counter+hemat ─► B4 Notif
       msgBus inject,                      │      (sediakan counter)   (baca counter)
       key constants)                      └─► [B1,B2 independen satu sama lain]
```

1. **B0 — Fondasi bersama** (prasyarat semua): `providerFromModel`, injeksi msgBus ke
   hook, konstanta key Redis, kerangka mount di `agent_init.go`.
2. **B1 Cache** & **B2 Rate-limit** — independen, boleh paralel setelah B0.
3. **B3 Counter+mode hemat** — menyediakan `kios:llm:usage:*`.
4. **B4 Notif** — **bergantung pada B3** (membaca counter). Kerjakan terakhir.
5. **A Utang** — sepenuhnya independen; tidak memblokir/diblokir B.

---

## 5. Spike pra-implementasi (WAJIB diverifikasi sebelum koding)

Hal-hal yang agen tidak bisa pastikan karena mode desain-saja. Verifikasi singkat dulu:

- **S1** — Field identitas user di `LLMHookRequest.Meta` (`HookMeta`): adakah chatID/
  senderID untuk rate-limit & target balasan? Cek `turn_context.go` / `hooks.go:93`.
- **S2** — Konvensi `providerFromModel`: pastikan pemetaan model_name→provider andal
  (cek `pkg/providers` + `entrypoint.sh model_list`).
- **S3** — Thread-safety `bus.PublishOutbound` dipanggil dari goroutine hook (#2,#3,#4
  publish saat `AbortTurn`). Cek `pkg/bus`.
- **S4** — Titik injeksi debounce (#3): level `MessageBus` atau handler
  `pkg/channels/telegram/`? Tentukan yang paling aditif.
- **S5** — Dukungan Lua `EVAL` di Upstash via go-redis/v9 (#3 rate-limit atomik).
  Bila tidak, pakai `INCR`+`EXPIRE` (sudah ada fallback di desain #3).
- ~~**S6** — Invalidasi cache lintas-sumber (#2).~~ **GUGUR** oleh keputusan §6.3
  (cache non-produk dulu): jawaban yang di-cache tidak bergantung data produk, jadi tidak
  ada masalah stale dari update dashboard. Tidak perlu endpoint invalidasi iterasi ini.

---

## 6. Keputusan (DIPUTUSKAN — 2026-05-30)

> Diputuskan sesuai rekomendasi. Dampak ke desain dicatat di tiap poin.

1. **Utang & stok → KURANGI STOK saat `catat` bon.** Bon = penjualan tertunda, jadi
   stok berkurang seketika. *Dampak:* `kios_hutang.catat` harus memanggil logika
   pengurangan stok yang sama dengan `kios_kasir.jual` (validasi `stok>=qty`, emit sinyal
   stok kritis), tapi TANPA mencatat transaksi tunai — total masuk ke piutang.
   `bayar`/`lunas` hanya menggerakkan uang, tidak menyentuh stok lagi. Revisi `01-utang.md`
   §integrasi-kasir saat implementasi.
2. **Utang & jatuh tempo → TUNDA.** Tidak ada field `jatuh_tempo`/reminder hutang lama di
   F1. Jaga tool ramping. Catat sebagai kandidat iterasi berikut (lihat ROADMAP).
3. **Cache FAQ → NON-PRODUK DULU.** Hanya cache pertanyaan yang jawabannya tidak
   bergantung data produk/harga/stok (mis. jam buka, lokasi, cara order). *Dampak:* **spike
   S6 (invalidasi lintas-sumber) GUGUR** — tidak perlu endpoint invalidasi dashboard
   untuk iterasi ini. `02-cache-faq.md` perlu menambah daftar "kriteria cacheable" yang
   mengecualikan intent terkait produk.
4. **Aktivasi → BERTAHAP, default OFF.** Semua env subsistem kuota default mati. Urutan
   nyala: audit limit provider nyata → set `KIOS_LLM_LIMIT_*` → aktifkan cache → rate-limit
   → counter/mode-hemat → notif. (Audit limit = item hardening di `tasks.md`.)

---

## 7. Kriteria penerimaan (termasuk Tes Beban — Fase 2.5)

Subsistem dianggap selesai bila:

- [ ] Cache: pertanyaan FAQ berulang dijawab tanpa panggilan LLM (verifikasi counter tidak naik).
- [ ] Rate-limit: 1 user spam > batas → dapat pesan ramah, user lain tidak terdampak.
- [ ] Mode hemat: saat `usage ≥ ambang`, pesan butuh-LLM dibalas ramah; slash command &
      cache tetap jalan.
- [ ] Counter: `INCR` hanya saat panggilan LLM sukses; reset di pergantian hari WITA.
- [ ] Notif: owner dapat notif sekali per ambang per hari per provider (tidak spam).
- [ ] **Tes beban**: simulasi banyak pesan barengan + simulasi kuota habis (key API
      dummy/invalid 1 provider) → bot **tidak crash**, degradasi mulus, tidak ada error
      mentah ke pembeli. (Memenuhi item "Tes beban" Fase 2.5 di `tasks.md`.)
- [ ] Semua hook aditif: `make build` & test picoclaw core tetap hijau.
- [ ] Unit test table-driven + miniredis untuk tiap fitur.

---

## 8. Status sub-task (ruflo)

| # | Sub-task | Status | Doc |
|---|----------|:--:|-----|
| 1 | Utang | ✅ desain | `01-utang.md` |
| 2 | Cache FAQ | ✅ desain | `02-cache-faq.md` |
| 3 | Rate-limit | ✅ desain | `03-rate-limit.md` |
| 4 | Counter+mode hemat | ✅ desain | `04-counter-mode-hemat.md` |
| 5 | Notif kuota | ✅ desain | `05-notif-kuota.md` |
| 6 | Gabungkan (dokumen ini) | ✅ selesai | `PLAN-fase2.5.md` |

**Langkah berikutnya:** user menjawab §6 → buat plan implementasi (writing-plans) →
mulai B0 + Jalur A.
