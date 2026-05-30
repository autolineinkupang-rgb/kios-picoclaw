# Desain 05: Notifikasi Kuota LLM ke Owner

**Versi:** 0.1 (desain awal, belum diimplementasi)
**Tanggal:** 2026-05-30
**Konteks subsistem:** Bagian dari "Manajemen Kuota LLM" — fitur #5 dari 5, dirancang paralel bersama:
- #2 Cache FAQ
- #3 Rate-limit per-user
- #4 Counter harian + mode hemat
- #5 Notif kuota ← dokumen ini

---

## 1. Tujuan dan Ruang Lingkup

**Tujuan:** Memberitahu owner kios sebelum dan saat layanan AI masuk mode hemat, sehingga owner bisa mengambil tindakan (isi ulang kuota, ganti provider, koordinasi dengan pengelola).

**Dalam ruang lingkup:**
- Membaca counter pemakaian harian per provider dari Redis (dihasilkan fitur #4).
- Memicu notifikasi Telegram ke owner pada ambang persentase bertingkat (misal 80% dan 100%/mode hemat).
- Dedup per ambang per hari per provider agar tidak spam.
- Integrasi ke loop `NotifService` yang sudah ada di `notif.go`.
- Env baru untuk mengaktifkan/menonaktifkan fitur dan mengatur ambang.

**Di luar ruang lingkup:**
- Intercept LLM call — ditangani fitur #3 dan #4.
- Counter increment — ditangani fitur #4.
- Cache FAQ — ditangani fitur #2.
- Notifikasi ke kasir atau pembeli — hanya owner yang relevan untuk kuota.

---

## 2. Data Model

### 2.1 Key yang Dikonsumsi (Kontrak dari Fitur #4)

```
kios:llm:usage:{provider}:{YYYY-MM-DD}   STRING   integer — jumlah token atau request hari ini
kios:llm:limit:{provider}                STRING   integer — batas harian yang dikonfigurasi
kios:llm:frugal_mode                     STRING   "1" bila mode hemat aktif, kosong/tidak ada bila normal
```

- `{provider}` adalah string lowercase identitas provider: `groq`, `gemini`, `anthropic`, dll.
- Tanggal dalam format `2006-01-02` (WITA), konsisten dengan konvensi `NowWITA()` yang sudah ada.
- Fitur ini HANYA membaca key di atas, tidak pernah menulis ke dalamnya.

**Asumsi kontrak** (lihat bagian 8 untuk pertanyaan terbuka):
1. Fitur #4 menyimpan counter sebagai integer STRING yang bisa dibaca dengan `GET` lalu `strconv.Atoi`.
2. Fitur #4 menyimpan batas di `kios:llm:limit:{provider}` atau menyediakan fungsi helper di package yang sama.
3. Fitur #4 menyimpan flag mode hemat di `kios:llm:frugal_mode` sebagai "1"/"".
4. Scope tanggal menggunakan zona waktu WITA (UTC+8), konsisten dengan `NowWITA()`.
5. Counter di-reset setiap hari oleh fitur #4 (bisa via TTL 25 jam atau INCR di hari baru).

### 2.2 Key Dedup yang Dikelola Fitur Ini

```
kios:llm:notif_sent:{provider}:{YYYY-MM-DD}:{level}   STRING   "1"
```

- `{level}` adalah string ambang yang sudah dikirim, misal: `"80"`, `"100"`.
- TTL: **25 jam** — sama dengan pola `keyNotifLastDate` di `notif.go` baris 148.
- Reset otomatis esok hari via TTL (tidak perlu logika reset manual).
- Satu key per kombinasi provider + tanggal + level, sehingga:
  - Groq 80% → `kios:llm:notif_sent:groq:2026-05-30:80`
  - Groq 100% → `kios:llm:notif_sent:groq:2026-05-30:100`
  - Gemini 80% → `kios:llm:notif_sent:gemini:2026-05-30:80`

### 2.3 Mengapa Tidak Menggunakan Hash Tunggal

Menggunakan STRING individual dengan TTL lebih sederhana dan konsisten dengan pola `keyNotifLastDate` yang sudah ada. HASH tidak bisa TTL per field di Redis standar.

---

## 3. Algoritma

### 3.1 Fungsi Utama: `tryNotifyKuota(ctx)`

Dipanggil dari loop `NotifService` setiap siklus ~2 menit.

```
tryNotifyKuota(ctx):
  jika env KIOS_LLM_NOTIF_ENABLED != "true":
    return

  ambang := parseAmbang(os.Getenv("KIOS_LLM_NOTIF_THRESHOLDS"))
  // default: [80, 100]

  providers := daftarProviderAktif()
  // baca dari env: GROQ_API_KEY → "groq", GEMINI_API_KEY → "gemini", dst.

  today := NowWITA().Format("2006-01-02")

  untuk setiap provider dalam providers:
    usage := bacaCounter(ctx, provider, today)    // GET kios:llm:usage:{provider}:{today}
    limit  := bacaLimit(ctx, provider)            // GET kios:llm:limit:{provider}
    jika limit == 0: skip (belum dikonfigurasi)

    persen := (usage * 100) / limit

    untuk setiap level dalam ambang (urut dari besar ke kecil):
      jika persen < level: skip
      dedupKey := fmt.Sprintf("kios:llm:notif_sent:%s:%s:%d", provider, today, level)
      jika redis.Exists(dedupKey): skip (sudah dikirim)

      msg := buildKuotaMessage(provider, persen, level, usage, limit)
      sendToOwners(ctx, msg)   // fungsi yang sudah ada di notif.go
      redis.Set(dedupKey, "1", 25*time.Hour)
      break  // satu notif per siklus per provider (level tertinggi yang belum dikirim)
```

**Catatan desain:** Loop `break` setelah kirim memastikan dalam satu siklus hanya satu level yang dikirim per provider. Level tertinggi yang terpicu selalu dikirim duluan (karena ambang diurutkan besar ke kecil).

### 3.2 Pembacaan Counter

```go
// readLLMUsage membaca counter pemakaian hari ini dari Redis.
// Mengembalikan 0 bila key tidak ada (belum ada pemakaian hari ini).
func readLLMUsage(ctx context.Context, rdb *redis.Client, provider, date string) int {
    key := fmt.Sprintf("kios:llm:usage:%s:%s", provider, date)
    v, err := rdb.Get(ctx, key).Result()
    if err != nil { return 0 }
    n, _ := strconv.Atoi(v)
    return n
}

// readLLMLimit membaca batas harian provider.
// Mengembalikan 0 bila belum dikonfigurasi (fitur nonaktif untuk provider ini).
func readLLMLimit(ctx context.Context, rdb *redis.Client, provider string) int {
    key := fmt.Sprintf("kios:llm:limit:%s", provider)
    v, err := rdb.Get(ctx, key).Result()
    if err != nil { return 0 }
    n, _ := strconv.Atoi(v)
    return n
}
```

### 3.3 Pemeriksaan Mode Hemat

Mode hemat dicek terpisah karena bisa aktif bahkan bila counter tidak mencapai 100% (misal ditetapkan manual oleh owner atau fitur #3 memutuskan masuk mode hemat lebih awal).

```
checkFrugalModeNotif(ctx):
  v, _ := redis.Get("kios:llm:frugal_mode")
  jika v != "1": return

  dedupKey := "kios:llm:notif_sent:frugal:" + today
  jika redis.Exists(dedupKey): return

  sendToOwners(ctx, buildFrugalModeMessage())
  redis.Set(dedupKey, "1", 25*time.Hour)
```

### 3.4 Format Pesan Bahasa Indonesia

**Ambang 80%:**
```
⚠️ *Peringatan Kuota LLM* — {tanggal} WITA

Provider *{PROVIDER}* sudah terpakai *{persen}%* hari ini ({usage}/{limit} request).

Layanan AI masih berjalan normal, tapi kuota menipis.
💡 Saran: pantau pemakaian atau siapkan provider cadangan.
```

**Ambang 100% / masuk mode hemat:**
```
🔴 *Kuota LLM Habis* — {tanggal} WITA

Provider *{PROVIDER}* sudah mencapai batas harian ({usage}/{limit} request).
Layanan AI sekarang dalam *mode hemat* — hanya perintah dasar yang tersedia.

💡 Saran tindakan:
• Tunggu reset kuota besok pagi
• Aktifkan provider cadangan (cek GEMINI_API_KEY / ANTHROPIC_API_KEY)
• Hubungi pengelola untuk tambah kuota

Pelanggan tetap bisa pakai /stok, /harga, /jual — tapi jawaban AI dinonaktifkan sementara.
```

**Mode hemat aktif (dari checkFrugalModeNotif):**
```
🟡 *Mode Hemat Aktif* — {tanggal} WITA

Asisten kios sekarang berjalan dalam mode hemat — AI dinonaktifkan sementara untuk menjaga layanan tetap tersedia.

Perintah yang masih berfungsi: /stok, /harga, /jual, /laporan
💡 Hubungi pengelola jika perlu bantuan lebih lanjut.
```

---

## 4. Titik Integrasi

### 4.1 File Baru: `pkg/tools/kios/notif_kuota.go`

File baru terpisah dari `notif.go` agar tetap di bawah 500 baris dan tanggung jawab tetap tunggal. Berisi:
- Struct/konstanta key dedup
- Fungsi `tryNotifyKuota(ctx)`
- Fungsi `checkFrugalModeNotif(ctx)`
- Fungsi `buildKuotaMessage(...)` dan `buildFrugalModeMessage()`
- Helper `readLLMUsage`, `readLLMLimit`, `daftarProviderAktif`

### 4.2 Modifikasi `notif.go` — `NotifService.loop()`

Tambahkan dua pemanggilan di dalam loop yang sudah ada:

```go
// Di dalam loop() — setelah tryNotifyPendingPileup(ctx):
n.tryNotifyKuota(ctx)
n.checkFrugalModeNotif(ctx)
```

Karena loop sudah berjalan tiap 2 menit dan memanggil beberapa `tryNotify*`, penambahan ini **aditif** dan tidak mengubah perilaku yang ada.

`NotifService` sudah memiliki akses ke `store.rdb` (via `store *Store`) dan `sendToOwners(ctx, msg)` — keduanya bisa langsung dipakai dari `notif_kuota.go` dalam package yang sama.

### 4.3 Tidak Ada Hook ke Counter

Keputusan: **Polling di loop, bukan hook saat increment.**

Alasan:
- Fitur #4 (counter) belum diimplementasi — kontraknya belum final. Menambahkan hook berarti coupling ketat.
- Loop 2 menit sudah ada dan dipakai untuk notif stok + pesanan. Latensi 2 menit sebelum notif kuota diterima sangat dapat diterima (bukan alerting realtime yang kritis).
- Polling lebih mudah diuji dengan miniredis (set key, panggil fungsi, assert notif terkirim).
- Tidak perlu modifikasi pada kode counter fitur #4 sama sekali — decoupled.

### 4.4 Target Pengiriman: Owner

Menggunakan `sendToOwners(ctx, msg)` yang sudah ada di `notif.go`. Fungsi ini sudah:
- Iterasi semua user dengan `Role == "owner"` dan `Aktif == true`.
- Pakai env `KIOS_OWNER_IDS` untuk bootstrap owner permanen.
- Tidak mengirim ke kasir.

Alasan tidak menggunakan `KIOS_REPORT_CHAT`: kuota adalah urusan operasional yang perlu segera diketahui owner secara personal, bukan laporan grup. Kirim langsung ke DM tiap owner lebih tepat. Bila `KIOS_OWNER_IDS` tidak diset dan tidak ada user owner di Redis, notif tidak terkirim — ini edge case yang didokumentasikan (lihat bagian 7).

---

## 5. Environment Variables Baru

| Variabel | Wajib | Default | Keterangan |
|----------|:-----:|---------|-----------|
| `KIOS_LLM_NOTIF_ENABLED` | — | `"false"` | Set `"true"` untuk aktifkan notif kuota. Default off sampai fitur #4 siap. |
| `KIOS_LLM_NOTIF_THRESHOLDS` | — | `"80,100"` | Daftar ambang persen, dipisah koma. Contoh: `"70,90,100"`. |

**Mengapa default off:** Fitur ini bergantung pada counter dari fitur #4. Tanpa counter, `readLLMLimit` selalu mengembalikan 0 dan tidak ada notif yang terkirim — tapi lebih aman eksplisit nonaktif sampai fitur #4 tersedia.

**Key Redis yang perlu diisi oleh fitur #4:**

| Key | Siapa yang menulis | Format |
|-----|-------------------|--------|
| `kios:llm:usage:{provider}:{YYYY-MM-DD}` | Fitur #4 | INTEGER string |
| `kios:llm:limit:{provider}` | Fitur #4 / setup awal | INTEGER string |
| `kios:llm:frugal_mode` | Fitur #4 | `"1"` atau tidak ada |

---

## 6. Rencana Test (Table-Driven + miniredis)

File: `pkg/tools/kios/notif_kuota_test.go`

### 6.1 Test: `TestShouldSendKuotaNotif`

Tabel kasus:
```
| usage | limit | level | dedup_exists | wantSend |
|-------|-------|-------|--------------|----------|
| 800   | 1000  | 80    | false        | true     | persen=80, belum terkirim
| 800   | 1000  | 80    | true         | false    | sudah terkirim (dedup)
| 700   | 1000  | 80    | false        | false    | persen=70, belum capai 80%
| 1000  | 1000  | 100   | false        | true     | persen=100, belum terkirim
| 1000  | 1000  | 80    | false        | true     | persen=100, notif 80 belum terkirim
| 0     | 0     | 80    | false        | false    | limit=0, fitur tidak dikonfigurasi
```

Unit test fungsi helper `shouldSendKuotaNotif(persen, level int, dedupExists bool) bool` — logika murni tanpa Redis.

### 6.2 Test: `TestTryNotifyKuota_Integration`

Menggunakan miniredis. Skenario:
1. Set `kios:llm:usage:groq:2026-05-30` = 850, `kios:llm:limit:groq` = 1000
2. Panggil `n.tryNotifyKuota(ctx)`
3. Assert: key dedup `kios:llm:notif_sent:groq:2026-05-30:80` ada di Redis
4. Panggil lagi — assert: notif tidak dikirim ulang (dedup aktif)

### 6.3 Test: `TestTryNotifyKuota_MultiProvider`

Skenario: Groq 85%, Gemini 92% — keduanya harus memicu notif 80%, dedup terpisah per provider.

### 6.4 Test: `TestTryNotifyKuota_DedupResetDaily`

Skenario:
1. Set dedup key dengan TTL kecil di miniredis
2. `miniredis.FastForward(26 * time.Hour)` — TTL expired
3. Panggil `tryNotifyKuota` lagi — assert: notif terkirim lagi (reset hari baru)

### 6.5 Test: `TestCheckFrugalModeNotif`

Skenario:
1. Set `kios:llm:frugal_mode` = "1"
2. Panggil `checkFrugalModeNotif(ctx)`
3. Assert dedup key `kios:llm:notif_sent:frugal:{today}` ada
4. Panggil lagi — assert: tidak kirim ulang

### 6.6 Test: `TestTryNotifyKuota_NoOwner`

Skenario: `kios:llm:usage:groq` = 900, limit = 1000, tapi tidak ada user owner di Redis dan `KIOS_OWNER_IDS` kosong. Assert: tidak ada panic, tidak ada error yang muncul ke permukaan (graceful no-op).

### 6.7 Test: `TestParseAmbang`

```
| input         | wantAmbang      |
|---------------|-----------------|
| ""            | [80, 100]       | default
| "70,90,100"   | [70, 90, 100]   |
| "abc"         | [80, 100]       | fallback ke default bila parse gagal
| "100"         | [100]           |
```

---

## 7. Edge Cases dan Risiko

### 7.1 Spam / Overnotifikasi

**Risiko:** Loop 2 menit bisa mengirim banyak notif bila dedup tidak bekerja.

**Mitigasi:** Key dedup dengan TTL 25 jam memastikan tiap (provider, hari, level) hanya satu notif. Ini identik dengan pola `keyNotifLastDate` yang sudah terbukti di `notif.go`.

### 7.2 Owner Tidak Dikonfigurasi

**Risiko:** Tidak ada user dengan `Role == "owner"` di Redis dan `KIOS_OWNER_IDS` kosong → notif tidak terkirim tanpa ada indikasi error.

**Mitigasi:** `sendToOwners` sudah mencatat log warn bila daftar user kosong. Tidak ada tindakan tambahan di lapisan notif kuota — ini tanggung jawab konfigurasi deployment. Didokumentasikan di CLAUDE.md sebagai prasyarat.

### 7.3 Banyak Provider Aktif

**Risiko:** 3 provider × 2 ambang = 6 potensi notif dalam satu siklus 2 menit.

**Mitigasi:** Dedup per provider mengurangi ini ke maksimal 1 notif baru per provider per siklus. Dalam praktik kios desa, biasanya hanya 1-2 provider aktif. Jika dirasa masih banyak, bisa tambahkan env `KIOS_LLM_NOTIF_COOLDOWN_MIN` di versi berikutnya.

### 7.4 Counter Tidak Ada / Fitur #4 Belum Deploy

**Risiko:** `readLLMUsage` dan `readLLMLimit` mengembalikan 0 → `persen = 0` → tidak ada notif.

**Mitigasi:** Ini perilaku yang diinginkan (default off). `KIOS_LLM_NOTIF_ENABLED` harus eksplisit `"true"` untuk aktif, dan fitur #4 harus sudah deploy sebelum env ini diset.

### 7.5 Ambang Tidak Unik / Urutan Tidak Beraturan

**Risiko:** Env `KIOS_LLM_NOTIF_THRESHOLDS=100,80` (urutan terbalik) atau duplikat `80,80,100`.

**Mitigasi:** `parseAmbang` mengurutkan descending dan deduplikasi nilai sebelum digunakan. Test 6.7 mencakup ini.

### 7.6 Overflow Integer

**Risiko:** Counter sangat besar (melebihi `int`) bila fitur #4 tidak membatasi increment.

**Mitigasi:** Gunakan `int64` untuk counter. Dalam konteks kios desa, batas harian Groq/Gemini berkisar ribuan request — tidak ada risiko overflow `int`.

---

## 8. Pertanyaan Terbuka (Ketergantungan ke Fitur #4)

### P1 — Format Counter (KRITIS)
Apakah fitur #4 menyimpan counter sebagai **jumlah request** atau **jumlah token**? Ini mempengaruhi satuan `limit` dan interpretasi persentase. Desain ini mengasumsikan **jumlah request** (paling mudah di-increment dan di-hitung). Jika token, perlu penyesuaian range limit (ribuan vs puluhan ribu).

### P2 — Nama Key Counter (KRITIS)
Desain ini mengasumsikan `kios:llm:usage:{provider}:{YYYY-MM-DD}`. Apakah fitur #4 memakai format ini persis? Jika berbeda (misal `kios:llm:{provider}:usage:{date}` atau pakai format lain), fungsi `readLLMUsage` harus diubah sebelum integrasi.

### P3 — Cara Simpan Limit (PENTING)
Apakah fitur #4 menyimpan limit di Redis (`kios:llm:limit:{provider}`) atau hanya dari env variable? Bila dari env, `readLLMLimit` perlu membaca env alih-alih Redis.

### P4 — Flag Mode Hemat (PENTING)
Apakah fitur #4 menggunakan key `kios:llm:frugal_mode` persis, atau nama lain? Apakah mode hemat per-provider atau global?

### P5 — Timezone Counter Reset (MODERAT)
Apakah fitur #4 menggunakan WITA untuk menentukan "hari ini" saat membuat key counter? Jika menggunakan UTC, counter hari ini di query WITA `2026-05-30` mungkin masih memakai key `2026-05-29` sampai pukul 08:00 WITA.

### P6 — Scope Provider (MODERAT)
Daftar provider aktif dideteksi dari env yang ada (`GROQ_API_KEY`, `GEMINI_API_KEY`, `ANTHROPIC_API_KEY`). Apakah fitur #4 mencatat provider dengan nama yang sama persis (`groq`, `gemini`, `anthropic`)? Perlu kesepakatan konstanta nama provider.

---

## Lampiran: Ringkasan Key Redis Terkait Fitur Ini

| Key | Tipe | Pemilik | Akses Fitur Ini |
|-----|------|---------|-----------------|
| `kios:llm:usage:{provider}:{YYYY-MM-DD}` | STRING | Fitur #4 | READ only |
| `kios:llm:limit:{provider}` | STRING | Fitur #4 | READ only |
| `kios:llm:frugal_mode` | STRING | Fitur #4 | READ only |
| `kios:llm:notif_sent:{provider}:{YYYY-MM-DD}:{level}` | STRING | Fitur #5 (ini) | READ + WRITE |
| `kios:llm:notif_sent:frugal:{YYYY-MM-DD}` | STRING | Fitur #5 (ini) | READ + WRITE |
