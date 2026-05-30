> PARKED — di luar scope KIOS_BUILD_SPEC.md. Jangan dikerjakan sampai 4 tool inti (stok/kasir/laporan/harga) sudah live. Spec tetap berlaku, TIDAK superseded.

# Design: Counter Pemakaian Harian + Mode Hemat (Fitur #4)

> Dokumen ini adalah desain arsitektur untuk implementor. Tidak ada kode yang ditulis di sini.
> Fitur ini adalah bagian dari subsistem "Manajemen Kuota LLM" yang dibangun paralel
> bersama: #2 Cache FAQ, #3 Rate-limit per-user, #5 Notif kuota.

---

## 1. Tujuan & Ruang Lingkup

### Tujuan
Memastikan layanan kios **tidak mati total** saat kuota LLM habis pada satu atau semua
provider. Turun anggun (graceful degradation): slash-command dan cache FAQ tetap
menjawab; pesan yang benar-benar butuh LLM mendapat balas ramah menjelaskan situasi.

### Ruang lingkup fitur ini (#4)
- Increment counter Redis setiap panggilan LLM sukses, per provider, per hari.
- Evaluasi apakah counter telah melewati ambang "mode hemat" sebelum LLM dipanggil.
- Ketika mode hemat aktif: mengembalikan `HookActionAbortTurn` dari `BeforeLLM` +
  memublikasikan balas ramah ke user via pipeline — LLM tidak dipanggil.
- Auto-reset pada pergantian hari (TZ Asia/Makassar, UTC+8).
- Menyediakan key & nilai yang dapat dibaca fitur #5 (Notif kuota).

### Di luar ruang lingkup
- Fallback antar-provider sudah ditangani `pkg/providers/error_classifier.go` — tidak diubah.
- Cache FAQ (fitur #2) dan rate-limit per-user (fitur #3) — didesain oleh agen lain;
  fitur ini hanya menentukan urutan relatifnya dalam hook.

---

## 2. Data Model

### 2.1 Key Redis: Counter Pemakaian

```
kios:llm:usage:{provider}:{YYYY-MM-DD}
```

- Tipe: `STRING` (integer, diinkremen dengan `INCR`)
- `{provider}`: salah satu dari `groq`, `gemini`, `claude` (lowercase, cocok dengan
  nama provider di `model_list` picoclaw)
- `{YYYY-MM-DD}`: tanggal dalam zona waktu **Asia/Makassar (UTC+8)**
- `EXPIRE`: 48 jam (2 hari). Memberikan ruang jika ada kebutuhan debug kemarin.
- Contoh key aktif: `kios:llm:usage:groq:2026-05-30`

Alasan per-provider, bukan agregat: limit harian Groq dan Gemini berbeda; mode hemat
mungkin hanya perlu aktif untuk Groq sementara Gemini masih tersisa.

### 2.2 Representasi "Mode Hemat": Dihitung On-the-fly

**Keputusan: mode hemat TIDAK disimpan sebagai state terpisah di Redis.**

Perbandingan dua pendekatan:

| Pendekatan | Kelebihan | Kekurangan |
|---|---|---|
| State eksplisit `kios:llm:mode_hemat` (STRING/HASH) | Sekali tulis, banyak baca | Butuh logika reset manual; bisa stale saat counter sudah turun |
| Dihitung on-the-fly dari counter | Selalu akurat; reset otomatis saat hari baru | 1 `GET` tambahan per panggilan LLM |

Pilihan: **on-the-fly**. Satu `GET` per panggilan tidak signifikan dibanding latensi
jaringan ke Groq/Gemini. Tidak ada stale state. Reset otomatis mengikuti TTL key.

### 2.3 Mode Hemat: Global vs Per-provider

**Keputusan: mode hemat bersifat per-provider, dikonfigurasi secara independen.**

Logika:
- Setiap provider memiliki limit dan persen-ambang sendiri.
- Saat `BeforeLLM` berjalan, hook memeriksa provider yang sedang akan dipanggil
  (`req.Model` → provider dinferensikan dari prefix atau konfigurasi aktif).
- Jika provider aktif dalam mode hemat → abort + balas ramah.
- Jika provider aktif masih aman tetapi provider lain sudah mode hemat → biarkan
  fallback chain core berjalan normal (tidak diintervensi hook ini).

Kasus khusus: jika **semua** provider dalam mode hemat → balas ramah generik
("layanan sedang istirahat").

### 2.4 Makassar Timezone

Tanggal key dihitung dari `time.Now().In(makassarLoc)` di mana `makassarLoc` adalah
`time.FixedZone("WITA", 8*3600)`. Tidak menggunakan `time.LoadLocation("Asia/Makassar")`
karena container Alpine mungkin tidak memiliki timezone database — fixed offset lebih
portabel.

---

## 3. Algoritma

### 3.1 Increment Counter (AfterLLM sukses)

```
fungsi IncrementUsage(ctx, provider):
    tanggal = nowMakassar().Format("2006-01-02")
    key = "kios:llm:usage:" + provider + ":" + tanggal
    INCR key                        // atomic, aman concurrent
    jika key baru (return value == 1):
        EXPIRE key 172800           // 48 jam dalam detik
```

Counter di-increment **hanya setelah panggilan LLM berhasil** (dalam `AfterLLM` hook
atau setelah `callLLM` sukses sebelum `AfterLLM`). Panggilan yang gagal/retry tidak
dicount untuk menghindari penalti ganda.

Alternatif yang dipertimbangkan: increment di `BeforeLLM`. Ditolak karena:
retry loop bisa increment berkali-kali untuk satu pesan user.

### 3.2 Evaluasi Ambang (BeforeLLM)

```
fungsi IsProviderInHematMode(ctx, provider) bool:
    limitHarian = env("KIOS_LLM_LIMIT_" + upper(provider), 0)
    if limitHarian == 0:
        return false               // fitur tidak aktif untuk provider ini

    persenAmbang = env("KIOS_LLM_HEMAT_PCT_" + upper(provider), 80)
    ambang = limitHarian * persenAmbang / 100

    tanggal = nowMakassar().Format("2006-01-02")
    key = "kios:llm:usage:" + provider + ":" + tanggal
    count = GET key (int, default 0 jika nil)
    return count >= ambang
```

### 3.3 Masuk Mode Hemat (BeforeLLM: abort)

Ketika `IsProviderInHematMode` mengembalikan `true` untuk provider yang sedang akan
dipanggil:

1. Hook mengembalikan `HookActionAbortTurn`.
2. Pipeline (`pipeline_llm.go` baris 109–117) menangkap `HookActionAbortTurn` dan
   mengeset `exec.abortedByHook = true`, lalu `return ControlBreak`.
3. Implementor harus memastikan pesan ramah ("Mode hemat...") dikirim. Ada dua opsi:
   - **Opsi A (rekomendasi)**: Hook memanipulasi `req` sebelum abort dengan menyuntikkan
     dummy response content, lalu gunakan `HookActionModify` + set `exec.finalContent`
     melalui mekanisme yang tersedia — tetapi pipeline saat ini tidak mendukung injeksi
     konten dari BeforeLLM secara langsung.
   - **Opsi B (lebih bersih)**: Gunakan `HookActionAbortTurn` dan dalam hook (sebelum
     return) publish outbound message langsung ke `bus.MessageBus` menggunakan chatID
     dari `req.Context.ChatID`. Ini setara dengan pola yang sudah dipakai di `notif.go`.
   - **Opsi C**: Implementasikan `KiosQuotaHook` sebagai `LLMInterceptor`. Dalam
     `BeforeLLM` set `req.Options["kios_hemat_reply"] = pesanRamah`, kemudian return
     `HookActionModify`. Lapis lain yang membaca Options ini kemudian mengirim reply.
     Terlalu kompleks, tidak disarankan.

   **Rekomendasi final: Opsi B** — publish outbound langsung di hook sebelum return
   `HookActionAbortTurn`. Hook perlu menerima referensi `*bus.MessageBus` saat
   konstruksi. Pattern ini sudah ada di `notif.go` (NotifService menggunakan msgBus
   yang sama).

### 3.4 Keluar Mode Hemat (Auto-reset)

Karena mode hemat dihitung on-the-fly dari counter, tidak ada "keluar" eksplisit.
Pada pergantian hari Makassar, tanggal berubah → key baru → counter mulai dari 0 →
provider otomatis keluar mode hemat.

Transisi terjadi tepat saat pesan pertama setelah tengah malam Makassar masuk.
Tidak ada goroutine tambahan yang diperlukan.

### 3.5 Identifikasi Provider dari BeforeLLM

`req.Model` berisi string model (contoh: `llama-3.1-8b-instant` atau `gemini-1.5-flash`).
Hook perlu menginferensikan provider dari model string ini. Strategi:
- Prefix match: `gemini-*` → `gemini`, `claude-*` → `claude`, sisanya → `groq`.
- Atau: baca `req.Context` yang memiliki field metadata turn; provider aktif bisa
  dicek dari `exec.activeProvider.Name()` tetapi itu tidak ada di `LLMHookRequest`.
- **Solusi paling sederhana**: tambahkan env `KIOS_LLM_MODEL_PROVIDER_MAP` (optional),
  atau gunakan prefix matching yang cukup untuk ketiga provider yang digunakan.

---

## 4. Titik Integrasi

### 4.1 File yang Terlibat

| File | Peran |
|---|---|
| `pkg/tools/kios/quota.go` (BARU) | Struct `KiosQuotaHook`, semua logika counter + evaluasi hemat |
| `pkg/agent/agent_init.go` | Mount hook ke `al.hooks` setelah kiosStore dibuat |
| `pkg/agent/hooks.go` | Interface `LLMInterceptor` (sudah ada, tidak diubah) |
| `pkg/agent/pipeline_llm.go` | Titik intersepsi `BeforeLLM` (line 82) dan `AfterLLM` (line 503) |

### 4.2 Urutan Evaluasi pada Hook Bersama

Semua fitur #2, #3, #4 menggunakan `HookManager.BeforeLLM` yang memanggil hook secara
berurutan berdasarkan `Priority` field. Urutan yang disepakati:

```
Priority 10: KiosCacheHook     (fitur #2 — cache FAQ)
             → jika cache hit: HookActionAbortTurn + reply dari cache
Priority 20: KiosRateLimitHook (fitur #3 — rate-limit per-user)
             → jika rate kena: HookActionAbortTurn + reply "coba lagi nanti"
Priority 30: KiosQuotaHook     (fitur #4 — counter + mode hemat)   <-- KITA
             → jika mode hemat: HookActionAbortTurn + reply ramah
Priority  - : [core provider fallback — berjalan dalam callLLM, bukan hook]
Priority  - : LLM dipanggil
```

Alasan urutan ini:
- Cache FAQ (10) paling murah dan paling menghemat kuota — harus dicek pertama.
- Rate-limit (20) melindungi dari spam user sebelum kita periksa kuota global.
- Mode hemat (30) adalah guard global terakhir sebelum LLM benar-benar dipanggil.

Hook diregistrasikan dengan `hm.Mount(HookRegistration{Name: "kios-quota", Priority: 30, Hook: quotaHook})` di `agent_init.go`.

### 4.3 Increment di AfterLLM

Dalam `AfterLLM` hook (Priority sama 30):
```
jika resp.Response != nil && resp.Response.FinishReason != "error":
    provider = inferProvider(resp.Model)
    IncrementUsage(ctx, provider)
```

Ini memastikan hanya panggilan yang benar-benar menghasilkan respons yang dicount.

### 4.4 Tidak Ada Perubahan Core

- `pipeline_llm.go` tidak diubah.
- `pipeline_finalize.go` tidak diubah.
- `hooks.go` tidak diubah.
- Semua tambahan bersifat aditif melalui `hm.Mount()`.

---

## 5. Env/Config Baru

### 5.1 Environment Variables

| Variabel | Default | Keterangan |
|---|---|---|
| `KIOS_LLM_LIMIT_GROQ` | `0` (disabled) | Limit panggilan harian untuk Groq. 0 = fitur nonaktif untuk provider ini |
| `KIOS_LLM_LIMIT_GEMINI` | `0` (disabled) | Limit harian Gemini |
| `KIOS_LLM_LIMIT_CLAUDE` | `0` (disabled) | Limit harian Claude |
| `KIOS_LLM_HEMAT_PCT_GROQ` | `80` | Persen ambang mode hemat Groq (dari limit). Mode hemat aktif saat usage >= limit * pct / 100 |
| `KIOS_LLM_HEMAT_PCT_GEMINI` | `80` | Persen ambang mode hemat Gemini |
| `KIOS_LLM_HEMAT_PCT_CLAUDE` | `80` | Persen ambang mode hemat Claude |
| `KIOS_LLM_QUOTA_ENABLED` | `true` | Master switch. `false` = hook tidak melakukan apa-apa (untuk debug) |

Contoh nilai production (Groq free tier ~14400 RPD, kios aktif ~12 jam):
```
KIOS_LLM_LIMIT_GROQ=500       # konservatif untuk kios desa
KIOS_LLM_HEMAT_PCT_GROQ=80    # mode hemat saat 400 panggilan
KIOS_LLM_LIMIT_GEMINI=200
KIOS_LLM_HEMAT_PCT_GEMINI=85
```

### 5.2 Kontrak Key Redis untuk Fitur #5 (Notif Kuota)

Fitur #5 (Notif kuota, didesain agen lain) HARUS membaca dari key berikut:

```
kios:llm:usage:{provider}:{YYYY-MM-DD}
```

Di mana:
- `{provider}` adalah `groq`, `gemini`, atau `claude`
- `{YYYY-MM-DD}` adalah tanggal dalam zona waktu Makassar (UTC+8)
- Nilai adalah integer string (hasil `INCR`)

Fitur #5 dapat menghitung sendiri persen pemakaian dengan membandingkan nilai key ini
terhadap env `KIOS_LLM_LIMIT_{PROVIDER}`.

**Ambang notifikasi yang disarankan untuk fitur #5:**
- Kirim notif warning saat usage mencapai **60%** dari limit (sebelum mode hemat di 80%)
- Kirim notif kritis saat usage mencapai ambang mode hemat (nilai `KIOS_LLM_HEMAT_PCT_*`)
- Kirim notif pemulihan saat hari baru dimulai (counter kembali ke 0)

Fitur #5 tidak perlu key Redis tambahan — cukup polling key usage di atas setiap N menit.

---

## 6. Rencana Test

Semua test table-driven menggunakan `miniredis` untuk mock Redis.

### 6.1 Unit Test `quota.go`

File: `pkg/tools/kios/quota_test.go`

**TestIncrementUsage**
| Kasus | Setup | Ekspektasi |
|---|---|---|
| Hari pertama, counter baru | Redis kosong | GET key = 1, EXPIRE ~48h |
| Panggilan kedua hari sama | Key sudah ada = 5 | GET key = 6 |
| Provider berbeda tidak saling mempengaruhi | Groq=10, Gemini=0 | Groq=11, Gemini=0 setelah increment Groq |
| Tanggal Makassar berubah | Jam 00:01 UTC+8 | Key baru dengan tanggal baru |

**TestIsProviderInHematMode**
| Kasus | Setup | Ekspektasi |
|---|---|---|
| Limit tidak dikonfigurasi (0) | KIOS_LLM_LIMIT_GROQ="" | false selalu |
| Di bawah ambang | limit=100, pct=80, count=79 | false |
| Tepat di ambang | limit=100, pct=80, count=80 | true |
| Melewati ambang | limit=100, pct=80, count=95 | true |
| Counter 0 (hari baru) | key tidak ada | false |
| Provider berbeda tidak saling pengaruhi | Groq hemat, Gemini tidak | hanya Groq true |

**TestAllProvidersInHematMode**
| Kasus | Ekspektasi |
|---|---|
| Semua 3 provider di atas ambang | true |
| 2 dari 3 di atas ambang | false |
| Semua tidak dikonfigurasi | false |

**TestInferProvider**
| Input model string | Ekspektasi provider |
|---|---|
| `llama-3.1-8b-instant` | `groq` |
| `gemini-1.5-flash` | `gemini` |
| `claude-3-haiku-20240307` | `claude` |
| `unknown-model` | `groq` (fallback default) |

### 6.2 Integration Test: Hook BeforeLLM

File: `pkg/tools/kios/quota_hook_test.go`

**TestKiosQuotaHookBeforeLLM**
| Kasus | Setup | Ekspektasi |
|---|---|---|
| Mode hemat tidak aktif | count < ambang | HookActionContinue, tidak ada pesan outbound |
| Mode hemat aktif, satu provider | count >= ambang | HookActionAbortTurn, pesan ramah dipublikasikan |
| Semua provider hemat | semua count >= ambang | HookActionAbortTurn, pesan "semua provider" |
| KIOS_LLM_QUOTA_ENABLED=false | apapun | HookActionContinue selalu |
| Context timeout | Redis lambat | HookActionContinue (fail open, jangan block user) |

### 6.3 Test Reset Harian

**TestDailyReset**
- Set counter Groq hari ini ke 500 (di atas ambang)
- Simulasikan pergantian hari dengan memajukan tanggal mock
- Verifikasi `IsProviderInHematMode` mengembalikan `false` (key baru = 0)

---

## 7. Edge Cases & Risiko

### 7.1 Race Condition pada INCR

`INCR` pada Redis adalah atomic — tidak ada race condition untuk counter itu sendiri.
Namun evaluasi "ambang" (GET → compare → keputusan) dan increment (INCR setelah LLM)
bukan satu transaksi atomik. Kemungkinan:
- N goroutine bersamaan bisa lolos ke LLM sebelum salah satu dari mereka increment.
- Ini **dapat diterima** untuk use case kios desa: over-shoot beberapa call tidak
  kritis, dan tidak ada uang yang ditransaksikan.
- Jika perlu lebih ketat: gunakan Lua script `EVAL` untuk atomic GET-compare-INCR.
  Tidak direkomendasikan untuk MVP — tambah kompleksitas tanpa nilai signifikan.

### 7.2 Provider yang Tidak Dikenali

Jika picoclaw core menambahkan provider baru yang belum ada di inferProvider():
- Fungsi harus mengembalikan provider default (`groq`) atau string `"unknown"`.
- Untuk `"unknown"`: limit tidak dikonfigurasi → `IsProviderInHematMode` → `false`.
- Fallback ke `groq` lebih berisiko (count masuk ke bucket salah).
- **Rekomendasi**: fallback ke `"unknown"`, sehingga panggilan tetap dihitung tapi
  mode hemat tidak diaktifkan — fail open, bukan fail closed.

### 7.3 Redis Tidak Tersedia

Jika operasi Redis gagal saat evaluasi:
- `IsProviderInHematMode` harus mengembalikan `false` (fail open) dengan log warning.
- Jangan block pipeline karena Redis down — user tetap bisa menggunakan LLM.
- Increment yang gagal (INCR error) juga: log warning, lanjutkan — counter sedikit
  under-count lebih baik daripada LLM call yang diblokir.

### 7.4 Pergantian Hari Saat Bot Berjalan

Tidak ada isu spesial: key menggunakan tanggal Makassar real-time. Bot yang berjalan
24 jam otomatis menggunakan key baru setelah tengah malam Makassar tanpa restart.

### 7.5 Mode Hemat Menghalangi Fitur Penting

Bahaya: mode hemat bisa menghalangi operasi owner yang sah (mis. laporan akhir hari,
kelola produk via AI). Mitigasi:
- Slash command (/laporan, /stok, dll.) sudah bypass LLM sepenuhnya via `commands.go`.
- Mode hemat hanya mempengaruhi path LLM, bukan slash command.
- Balas ramah harus menyebut bahwa slash command masih tersedia.

### 7.6 Estimasi Ukuran Data Redis

Per provider per hari: 1 key STRING, nilai integer kecil (~10 byte), TTL 48 jam.
Maksimal 3 provider × 2 hari = 6 key aktif. Tidak ada dampak storage yang berarti.

---

## 8. Pertanyaan Terbuka

**Q1: Apakah hook boleh menerima `*bus.MessageBus` langsung, atau harus ada abstraksi?**

Saat ini `notif.go` menerima `*bus.MessageBus` langsung. Pattern yang sama dapat dipakai
untuk `KiosQuotaHook`. Namun idealnya hook menggunakan interface `MessagePublisher` agar
lebih mudah di-mock dalam test. Implementor perlu memutuskan ini sebelum coding.

**Q2: Bagaimana inferProvider dari model string akan dipelihara?**

Prefix matching (`strings.HasPrefix`) rentan jika provider mengganti nama model. Apakah
perlu env `KIOS_LLM_PROVIDER_FOR_{MODEL_PREFIX}` yang lebih fleksibel, atau cukup
hardcode untuk 3 provider saat ini? Untuk MVP: hardcode cukup.

**Q3: Apakah fitur #5 (Notif kuota) akan polling aktif atau subscribe event?**

Fitur ini tidak menghasilkan event; fitur #5 harus polling key Redis secara periodik
(seperti `notif.go` yang sudah ada). Konfirmasi dengan desainer fitur #5 bahwa
polling 2–5 menit cukup untuk tujuan notifikasi.

**Q4: Apakah mode hemat perlu dapat di-override manual oleh owner?**

Skenario: owner ingin memaksa LLM aktif kembali meski counter sudah tinggi (mis. ada
situasi darurat). Opsi: slash command `/kuota reset` yang clear counter atau set env
`KIOS_LLM_QUOTA_ENABLED=false` sementara. Belum diputuskan — pertimbangkan untuk
milestone berikutnya.

**Q5: Siapa yang menentukan "provider aktif" saat BeforeLLM dipanggil?**

`req.Model` tersedia di `LLMHookRequest`, tapi tidak ada field `Provider`. Inferensi
dari model string adalah heuristik. Alternatif: minta desainer fitur #2/#3 menyepakati
konvensi metadata di `req.Options["kios_provider"]` yang di-set oleh hook sebelumnya,
atau tambahkan field ke `LLMHookRequest` (perubahan core kecil tapi aditif).

---

## Lampiran: Pesan Ramah untuk User

```
// Mode hemat satu provider (tapi masih ada fallback)
"Maaf kak, asisten lagi istirahat sebentar karena paket harian hampir habis 🙏
Perintah cepat masih bisa dipakai ya: /stok, /harga, /jual, /laporan.
Coba lagi setelah tengah malam nanti atau besok pagi kak!"

// Semua provider dalam mode hemat
"Maaf kak, asisten lagi istirahat penuh hari ini — paket AI sudah habis 🙏
Perintah cepat tetap jalan: /stok, /harga, /jual, /laporan.
Besok pagi asisten sudah segar kembali ya kak!"
```
