> PARKED — di luar scope KIOS_BUILD_SPEC.md. Jangan dikerjakan sampai 4 tool inti (stok/kasir/laporan/harga) sudah live. Spec tetap berlaku, TIDAK superseded.

# Desain Fitur: Cache FAQ (jawaban 0-token)

> **Scope:** Subsistem "Manajemen Kuota LLM" — Fitur #2 (Cache FAQ)
> **Paralel dengan:** #3 Rate-limit per-user, #4 Counter harian + mode hemat, #5 Notif kuota
> **Status:** Dokumen desain — JANGAN implementasi sebelum semua 4 desain selesai

---

## 1. Tujuan & Ruang Lingkup

### Tujuan

Menjawab pertanyaan FAQ berulang (mis. "kios buka jam berapa", "ada produk apa saja") langsung dari Redis **tanpa memanggil LLM**, sehingga menghemat kuota Groq/Gemini secara signifikan.

Efek yang diharapkan: pertanyaan FAQ yang berulang = **0 token** per hit. Berdasarkan pola kios desa, diperkirakan 20-30% pesan non-command akan memenuhi syarat cacheable.

### Ruang Lingkup

- Normalisasi teks → hashing → lookup Redis sebelum LLM dipanggil.
- Cache write-through: jawaban LLM disimpan ke cache setelah berhasil.
- Invalidasi cache saat konfigurasi kios berubah.
- Kriteria cacheable: membedakan pertanyaan statis vs dinamis/per-user.
- Env untuk kontrol TTL, on/off, dan max entries.
- Semua logika baru berada di `pkg/tools/kios/` — **tidak mengubah core picoclaw**.

### Di Luar Ruang Lingkup

- Tidak mengganti slash-command layer yang sudah ada (lapis 1 D7).
- Tidak menangani rate-limit per-user (fitur #3).
- Tidak menangani counter harian/mode hemat (fitur #4).
- Tidak meng-cache hasil tool call (hanya plain-text FAQ answer dari LLM).

---

## 2. Data Model

### Redis Keys

| Key | Tipe Redis | Format Value | TTL |
|-----|-----------|--------------|-----|
| `kios:cache:faq:{sha256_8hex}` | STRING | JSON `FAQEntry` | `KIOS_FAQ_TTL` (default 24 jam) |
| `kios:cache:faq:version` | STRING | integer (unix timestamp) | none (permanent) |

`sha256_8hex` = 8 karakter hex pertama dari SHA-256 dari normalized query.

**Mengapa 8 hex (32-bit)?** Ruang FAQ kios kecil (ratusan varian), collision risk <0.01% untuk N<1000 keys. Jika collision terdeteksi saat membaca (hash sama, pertanyaan berbeda), fallback ke LLM — lihat edge case section.

### Struct `FAQEntry`

```go
// FAQEntry menyimpan satu entri cache FAQ di Redis.
type FAQEntry struct {
    Hash        string `json:"hash"`         // 8-char SHA256 hex
    QuerySample string `json:"query_sample"` // representative normalized query
    Answer      string `json:"answer"`       // jawaban yang sudah di-format Bahasa Indonesia
    CreatedAt   int64  `json:"created_at"`   // Unix timestamp (untuk monitoring)
    HitCount    int64  `json:"hit_count"`    // counter cache hit (informational)
    Version     int64  `json:"version"`      // salin kios:cache:faq:version saat tulis
}
```

`version` digunakan untuk invalidasi generasional: jika `entry.Version < currentVersion`, entry dianggap stale meski belum expired.

### Namespace Constants (tambahkan ke `store.go`)

```go
const (
    keyFAQPrefix   = "kios:cache:faq:"        // prefix per-entry
    keyFAQVersion  = "kios:cache:faq:version" // generation counter
)
```

---

## 3. Algoritma

### 3.1 Normalisasi Query

```
normalize(s) =
  1. ToLower(s)
  2. Hapus tanda baca: regexp [^a-z0-9\s] → ""
  3. TrimSpace + collapse whitespace (regexp \s+ → " ")
  4. Truncate 200 chars (cegah query sangat panjang masuk cache)
```

Contoh:
- Input: `"Kios buka jam berapa??"` → normalized: `"kios buka jam berapa"`
- Input: `"  Ada BERAS gak??  "` → normalized: `"ada beras gak"`

### 3.2 Hash Key

```
hash8 = hex(SHA256(normalized_query))[:8]
redis_key = "kios:cache:faq:" + hash8
```

Implementasi Go: `crypto/sha256` (sudah tersedia di stdlib).

### 3.3 Kriteria Cacheable

**BOLEH di-cache** (FAQ statis / semi-statis):
- Pertanyaan tentang jam operasional (dikontrol `KiosConfig`)
- Pertanyaan tentang info umum kios (nama, lokasi, kontak)
- Daftar kategori produk umum
- Cara bayar / metode pembayaran
- Pertanyaan tentang QRIS
- Pertanyaan panduan penggunaan bot
- Pertanyaan harga produk spesifik (karena harga berubah via `SaveConfig`/`SetProduk` — dihapus via invalidasi versi)

**TIDAK boleh di-cache** (dinamis / per-user / sensitif):
- Pertanyaan yang mengandung angka transaksi/ID (mis. "TRX-0042")
- Pertanyaan yang mengandung nama orang / "aku", "saya", "kamu"
- Pertanyaan tentang utang / kredit per-pelanggan
- Pertanyaan yang mengandung tanggal spesifik ("kemarin", "hari ini") 
- Apapun yang menyebut session atau riwayat percakapan
- Pesan pendek (<10 karakter normalized) — terlalu ambigu

**Implementasi kriteria** — fungsi `isCacheable(normalized string) bool`:

```go
var noCachePatterns = []*regexp.Regexp{
    regexp.MustCompile(`\btrx[-\s]?\d+\b`),           // transaction ID
    regexp.MustCompile(`\b(aku|saya|kamu|anda)\b`),   // personal pronouns
    regexp.MustCompile(`\b(kemarin|tadi|tadi pagi|minggu lalu)\b`), // relative time
    regexp.MustCompile(`\butang\b`),                   // credit/debt query
    regexp.MustCompile(`\b\d{4,}\b`),                  // long numbers (IDs, phone)
}

func isCacheable(normalized string) bool {
    if len(normalized) < 10 {
        return false
    }
    for _, re := range noCachePatterns {
        if re.MatchString(normalized) {
            return false
        }
    }
    return true
}
```

### 3.4 Alur Baca (Cache Lookup)

```
IncomingMessage
    │
    ▼
[KiosFAQCache.Get(ctx, rawQuery)]
    │
    ├── normalize(rawQuery) → jika len < 10 atau !isCacheable → MISS
    │
    ├── hash8 = SHA256[:8]
    │
    ├── Redis GET kios:cache:faq:{hash8}
    │       │
    │       ├── err=redis.Nil → MISS
    │       │
    │       └── found → parse FAQEntry
    │               │
    │               ├── entry.QuerySample != normalized (collision check) → MISS + log
    │               │
    │               └── entry.Version < currentVersion → MISS (stale, delete async)
    │
    ├── HIT → return entry.Answer, nil
    │
    └── MISS → return "", nil
```

### 3.5 Alur Tulis (Cache Write-Through)

Dipanggil setelah LLM berhasil menjawab:

```
[KiosFAQCache.Put(ctx, rawQuery, answer)]
    │
    ├── isCacheable check → jika false → skip
    │
    ├── currentVersion = Redis GET kios:cache:faq:version
    │
    ├── entry = FAQEntry{hash, normalized, answer, now, 0, currentVersion}
    │
    ├── maxEntries check: jika DBSIZE (prefix scan) >= KIOS_FAQ_MAX_ENTRIES → skip
    │       (lihat catatan: SCAN cursor atau counter terpisah)
    │
    └── Redis SET kios:cache:faq:{hash8} JSON(entry) EX {ttl_seconds}
```

### 3.6 Invalidasi Cache

Saat data yang mempengaruhi FAQ berubah, **increment version counter**:

```go
// InvalidateFAQCache menaikkan versi sehingga semua cache FAQ dianggap stale.
func (s *Store) InvalidateFAQCache(ctx context.Context) error {
    return s.rdb.Incr(ctx, keyFAQVersion).Err()
}
```

**Trigger invalidasi** (dipanggil dari dalam Store methods yang sudah ada):

| Method yang dimodifikasi | Alasan |
|--------------------------|--------|
| `Store.SaveConfig()` | Jam buka, nama kios, QRIS berubah |
| `Store.SetProduk()` | Harga/stok produk berubah |
| `Store.DelProduk()` | Produk dihapus |

Modifikasi bersifat minimal: cukup tambahkan `s.InvalidateFAQCache(ctx)` di akhir masing-masing method (error invalidasi diabaikan / log warning — tidak blokir operasi utama).

Invalidasi menggunakan **generational versioning** (bukan DEL semua key) karena:
- Tidak perlu SCAN atau KEYS * yang mahal di Upstash
- Versi lama otomatis expire via TTL
- O(1) — hanya satu INCR + satu GET

---

## 4. Titik Integrasi

### 4.1 Lokasi Hook — TIDAK di picoclaw core

Arsitektur picoclaw menyediakan `LLMInterceptor` interface (`BeforeLLM` / `AfterLLM` di `pkg/agent/hooks.go`) yang bisa di-mount via `HookManager.Mount()`. **Namun**, hook ini berada di layer core agent dan membutuhkan akses ke `AgentLoop`, sehingga pendekatan yang lebih aditif adalah membuat **middleware di sisi kios** sebelum teks user diteruskan ke agent.

**Hook point yang dipilih: `pkg/tools/kios/faq_cache.go` + integrasi di `pkg/agent/agent_init.go`**

Lebih tepatnya: mount `KiosFAQHook` sebagai `LLMInterceptor` ke `HookManager` picoclaw. Ini adalah titik resmi yang didukung core tanpa modifikasi:

```
pkg/agent/agent_init.go → NewAgentLoop()
    └── [setelah al.hooks = NewHookManager(...)]
        └── if kiosStore != nil {
                faqCache := kios.NewFAQCache(kiosStore)
                al.hooks.Mount(agent.NamedHook("kios-faq-cache", faqCache))
            }
```

**Hook membaca user message dari `LLMHookRequest.Messages`** — pesan user terakhir ada di `messages[len-1]` dengan `Role == "user"`.

### 4.2 Implementasi `LLMInterceptor` oleh `KiosFAQHook`

```go
// pkg/tools/kios/faq_cache.go

type KiosFAQHook struct {
    cache   *FAQCache
    lastKey string // hash dari query yang sedang diproses (untuk AfterLLM)
}

// BeforeLLM: cek cache sebelum panggil LLM
func (h *KiosFAQHook) BeforeLLM(ctx context.Context, req *agent.LLMHookRequest) (*agent.LLMHookRequest, agent.HookDecision, error) {
    // 1. Ambil pesan user terakhir
    // 2. Lookup cache
    // 3. HIT → kembalikan HookActionAbortTurn + inject synthetic response
    //    MISS → kembalikan HookActionContinue, simpan hash untuk AfterLLM
}

// AfterLLM: simpan jawaban LLM ke cache jika applicable
func (h *KiosFAQHook) AfterLLM(ctx context.Context, resp *agent.LLMHookResponse) (*agent.LLMHookResponse, agent.HookDecision, error) {
    // Jika lastKey != "" → tulis ke cache
    // Selalu return HookActionContinue
}
```

**Catatan penting tentang `HookActionAbortTurn`:** Saat cache HIT, hook mengembalikan `HookActionAbortTurn`. Namun ini menyebabkan turn terhenti *tanpa* mengirim reply ke user. Perlu mekanisme inject jawaban cache ke output. Dua opsi:

- **Opsi A (Direkomendasikan):** Ubah `exec.response` (via `HookActionModify`) dengan membuat synthetic `LLMResponse{Content: cachedAnswer}` — hook `AfterLLM` tidak perlu karena BeforeLLM mengembalikan modified request, dan pipeline memperlakukannya sebagai direct LLM response. TAPI ini berarti BeforeLLM harus *mensimulasikan* response padahal itu AfterLLM territory.

- **Opsi B (Direkomendasikan setelah teliti baca kode):** Hook `BeforeLLM` mengembalikan `HookActionAbortTurn`. Kemudian hook juga implement `RuntimeEventObserver` yang mendengar event `KindAgentLLMRequest` dan mempublikasikan reply langsung ke bus. Ini lebih kompleks.

- **Opsi C (Paling sederhana, DIPILIH):** Hook `BeforeLLM` *tidak* digunakan untuk cache FAQ. Sebaliknya, intercept di **level lebih awal** — sebelum `runTurn` dipanggil, di `AgentLoop.handleInbound()` atau equivalennya. Di kios, ini berarti menambahkan middleware di handler inbound message sebelum turn dimulai, serupa dengan cara `commands.go` short-circuit dengan `req.Reply()`.

**Revisi titik hook setelah analisis kode lebih dalam:**

Membaca `pipeline_llm.go` line 81-119: `BeforeLLM` dengan `HookActionAbortTurn` menyebabkan `exec.abortedByHook = true` dan `ControlBreak` — turn berakhir tapi `finalContent` kosong. Tidak ada mekanisme "kembalikan konten dari hook" di BeforeLLM.

**Titik hook final yang dipilih: kios-side middleware sebelum agent turn**

Pola yang sama dengan `commands.go`: tambahkan pre-processing di layer Telegram handler, sebelum message diteruskan ke picoclaw agent. Ini 100% aditif dan tidak menyentuh core sama sekali.

Implementasi konkret:

```
pkg/tools/kios/faq_middleware.go  ← NEW FILE
```

Struct `FAQMiddleware` wrap fungsi `HandleMessage(ctx, chatID, text) (handled bool, reply string)`. Di-inject ke point registrasi command di `agent_init.go`:

```
pkg/agent/agent_init.go → NewAgentLoop()
    └── jika kiosStore != nil:
        faqMW := kios.NewFAQMiddleware(kiosStore)
        // Mount sebagai pre-command handler ATAU
        // inject ke kiosCmds sebagai Definition khusus dengan
        // Pattern matching (bukan slash, tapi teks biasa)
```

**Problem:** picoclaw tidak memiliki "catch-all non-slash handler" yang dapat di-inject dari luar. Slash commands di `commands.go` hanya cocok `/command`. Plain text langsung ke LLM via `runTurn`.

**Solusi akhir (setelah membaca arsitektur penuh):** Gunakan `BeforeLLM` hook dengan mekanisme yang benar:

`BeforeLLM` dapat mengembalikan `HookActionModify` dengan `LLMHookRequest` yang dimodifikasi. Karena hook tidak bisa "inject" jawaban langsung, kita **gunakan LLMInterceptor + HookActionAbortTurn DIKOMBINASIKAN dengan publishing outbound message secara langsung via `bus.PublishOutbound`**.

Ini memerlukan akses ke `msgBus` di dalam hook. Solusinya: inject `msgBus` saat konstruksi hook, dan ekstrak `chatID`/`channel` dari `LLMHookRequest.Context` (field `TurnContext`).

**Titik hook FINAL (definitif):**

```
File: pkg/tools/kios/faq_hook.go
Struct: KiosFAQHook implements agent.LLMInterceptor
Mount: pkg/agent/agent_init.go → NewAgentLoop(), setelah al.hooks dibuat
```

```go
// Di NewAgentLoop(), setelah al.hooks = NewHookManager(...):
if kiosStore != nil {
    faqHook := kios.NewFAQHook(kiosStore, msgBus)
    al.hooks.Mount(agent.NamedHook("kios-faq", faqHook))
}
```

`BeforeLLM` logic:
1. Ambil user message terakhir dari `req.Messages`
2. Lookup cache
3. Cache MISS → `HookActionContinue`, simpan context hash di goroutine-safe map
4. Cache HIT → publish reply langsung ke `msgBus` menggunakan `req.Context.ChatID` dan `req.Context.Channel`, lalu return `HookActionAbortTurn`

`AfterLLM` logic:
1. Ambil `req.Context` hash (jika MISS tadi)
2. Simpan `resp.Response.Content` ke cache jika `isCacheable`
3. Return `HookActionContinue`

State per-turn (hash dari query) disimpan menggunakan `sync.Map` keyed by `turnID` dari `req.Meta.TurnID`.

### 4.3 Urutan Komposisi dengan Fitur Kuota Lain

Urutan eksekusi `BeforeLLM` hooks ditentukan oleh `HookRegistration.Priority` (rendah = lebih dahulu):

```
Priority 10: kios-faq            ← Cache FAQ (FITUR INI)
             Jika HIT → abort, tidak ada LLM call sama sekali
             Fitur lain (#3, #4, #5) tidak perlu jalan

Priority 20: kios-ratelimit      ← Rate-limit per-user (#3)
             Hanya jalan jika FAQ miss

Priority 30: kios-daily-budget   ← Counter harian + mode hemat (#4)
             Increment usage counter SEBELUM LLM dipanggil
             Jika limit → abort dengan pesan ramah

Priority 40: [LLM dipanggil]
```

`AfterLLM` hooks (setelah LLM berhasil):
```
Priority 10: kios-faq            ← Simpan jawaban ke cache
Priority 30: kios-daily-budget   ← Catat token usage
Priority 50: kios-notif-kuota    ← Cek threshold, kirim notif (#5)
```

### 4.4 File-file yang Terlibat

| File | Action | Keterangan |
|------|--------|-----------|
| `pkg/tools/kios/faq_hook.go` | CREATE | `KiosFAQHook`, `FAQCache`, normalisasi, hash |
| `pkg/tools/kios/store.go` | MODIFY | Tambah `keyFAQPrefix`, `keyFAQVersion`, `InvalidateFAQCache()` |
| `pkg/tools/kios/harga.go` | MODIFY | Panggil `InvalidateFAQCache` setelah update harga |
| `pkg/agent/agent_init.go` | MODIFY | Mount `KiosFAQHook` ke `al.hooks` |

Perubahan `agent_init.go` minimal: 3 baris kode, aditif sepenuhnya.

---

## 5. Env / Config Baru

| Variabel | Default | Keterangan |
|----------|---------|-----------|
| `KIOS_FAQ_CACHE` | `true` | `false` untuk mematikan cache FAQ |
| `KIOS_FAQ_TTL` | `86400` | TTL cache dalam detik (default 24 jam) |
| `KIOS_FAQ_MAX_ENTRIES` | `500` | Maksimum entry dalam cache (soft limit) |

Dibaca via `os.Getenv` di konstruktor `NewFAQCache`. Tidak memerlukan perubahan di `KiosConfig` (config Redis) karena ini adalah env operasional bukan preferensi user.

`KIOS_FAQ_MAX_ENTRIES` adalah **soft limit**: jika jumlah key (dihitung via counter Redis terpisah di `kios:cache:faq:count`) sudah mencapai limit, entry baru tidak ditambahkan. Counter diincrement saat tulis dan tidaklah akurat sempurna (eventual consistency), tapi cukup untuk mencegah unbounded growth. Alternatif lebih sederhana: tidak enforce max entries, cukup andalkan TTL — dipertimbangkan sebagai default behavior jika `KIOS_FAQ_MAX_ENTRIES=0`.

---

## 6. Rencana Test

File test: `pkg/tools/kios/faq_hook_test.go`

Gunakan `miniredis` (konsisten dengan pola test kios yang sudah ada).

### Table-Driven Test Cases

```go
// TestNormalize
var normalizeTests = []struct {
    input    string
    expected string
}{
    {"Kios buka jam berapa??", "kios buka jam berapa"},
    {"  Ada BERAS gak???  ", "ada beras gak"},
    {"harga   minyak  goreng", "harga minyak goreng"},
    {"", ""},
    {"!!??", ""},
}

// TestIsCacheable
var cacheableTests = []struct {
    input    string
    expected bool
}{
    {"kios buka jam berapa", true},
    {"ada beras gak", true},
    {"trx-0042 sudah dicatat", false},   // transaction ID
    {"saya mau tanya", false},            // personal pronoun
    {"kemarin saya beli apa", false},     // relative time
    {"hi", false},                        // too short
    {"utang berapa", false},              // debt query
}

// TestFAQCacheHitMiss
func TestFAQCacheHitMiss(t *testing.T) {
    // Setup miniredis
    // Put entry
    // Get same query → HIT
    // Get different query → MISS
    // Get after version increment → MISS (stale)
    // Get after TTL expire (miniredis FastForward) → MISS
}

// TestFAQCacheHashCollision
func TestFAQCacheHashCollision(t *testing.T) {
    // Manually insert entry dengan hash = sha256("query A")[:8]
    // tapi QuerySample = "query B" (simulate collision)
    // Get("query A") → MISS (collision detected)
}

// TestFAQCacheInvalidateOnConfigSave
func TestFAQCacheInvalidateOnConfigSave(t *testing.T) {
    // Put entry
    // SaveConfig → version increment
    // Get same query → MISS
}

// TestFAQCacheMaxEntries (jika diimplementasikan)
func TestFAQCacheMaxEntries(t *testing.T) {
    // Set KIOS_FAQ_MAX_ENTRIES=3
    // Put 3 entries → semua berhasil
    // Put entry ke-4 → di-skip
}
```

### Integration Test (opsional, bisa di-skip di CI tanpa Redis)

- Mount `KiosFAQHook` ke mock `HookManager`
- Kirim `LLMHookRequest` dengan user message FAQ
- Verifikasi `HookActionContinue` pada MISS pertama
- Simulasikan `AfterLLM` dengan answer
- Kirim `LLMHookRequest` yang sama → verifikasi `HookActionAbortTurn` + outbound message

---

## 7. Edge Cases & Risiko

### 7.1 Stale Answer

**Risiko:** Jawaban yang di-cache menjadi tidak akurat setelah data berubah (mis. jam buka diupdate).

**Mitigasi:**
- Generational versioning: `InvalidateFAQCache()` dipanggil otomatis saat `SaveConfig`, `SetProduk`, `DelProduk`.
- TTL 24 jam sebagai safety net.
- Jika admin mengubah jam buka lewat Telegram (AI tool), `SaveConfig` selalu dipanggil → invalidasi terjadi.

**Gap:** Jika harga produk berubah via dashboard web (Next.js Server Action) yang tidak melalui bot Go, `SetProduk` di store.go bot tidak dipanggil. Cache bot akan stale sampai TTL expire.

**Solusi jangka panjang:** Dashboard web perlu memanggil endpoint invalidasi setelah update produk (via `KIOS_SERVICE_SECRET`). Di luar scope fitur ini — dicatat sebagai pertanyaan terbuka.

### 7.2 Hash Collision

**Risiko:** Dua pertanyaan berbeda menghasilkan hash 8 hex yang sama.

**Probabilitas:** Untuk N=500 entries, P(collision) ≈ N²/(2×16⁸) ≈ 500²/(2×4.3×10⁹) ≈ 0.003%. Sangat rendah.

**Mitigasi:** Collision detection di Get: bandingkan `entry.QuerySample` dengan `normalized`. Jika berbeda → MISS. Jawaban LLM yang benar disimpan, entry lama tertimpa.

### 7.3 Cache Poisoning

**Risiko:** Jawaban LLM yang salah/tidak akurat tersimpan dan di-serve berulang.

**Mitigasi:**
- TTL 24 jam membatasi window kerusakan
- `InvalidateFAQCache()` bisa dipanggil manual oleh owner: perintah `/invalidasicache` (opsional — bisa slash command baru yang memanggil `store.InvalidateFAQCache`)
- Cache hanya menyimpan `ForUser`/`Content` jawaban, bukan tool results — mengurangi surface area
- Jika jawaban LLM kosong atau error → tidak disimpan ke cache

### 7.4 Concurrency / Race Condition

**Risiko:** Dua request concurrent untuk query yang sama, keduanya MISS, keduanya panggil LLM, keduanya tulis cache.

**Mitigasi:** Redis SET adalah atomic. Double-write tidak berbahaya — entry terakhir menang. Tidak diperlukan distributed lock untuk use case ini.

### 7.5 Performa Lookup

**Risiko:** Setiap pesan (bahkan yang tidak akan di-cache) mengalami overhead Redis GET.

**Mitigasi:**
- Redis GET adalah O(1), round-trip Upstash dari Railway ~5-15ms
- Normalisasi + isCacheable check dilakukan sebelum Redis GET — untuk pesan non-cacheable (punya kata "saya", "trx-", dll.) → **tidak ada Redis call sama sekali** (early return MISS)
- Hook timeout default picoclaw = 5 detik (`defaultHookInterceptorTimeout`) — cukup aman

### 7.6 Dependency Injeksi `msgBus` ke Hook Kios

**Risiko:** `KiosFAQHook` perlu `msgBus` untuk publish reply saat cache HIT, tapi `msgBus` adalah struct core picoclaw.

**Mitigasi:** Inject `bus.MessageBus` (atau interface minimal `MessagePublisher`) saat konstruksi hook di `agent_init.go`. Pola ini sudah ada: `NotifService` juga menerima `msgBus` sebagai parameter.

Alternatif lebih clean: definisikan interface lokal `OutboundPublisher` di `pkg/tools/kios/` dengan satu method `PublishOutbound(ctx, msg)` → dependency tidak bergantung langsung ke `pkg/bus`.

---

## 8. Pertanyaan Terbuka

### Q1: Sinkronisasi invalidasi dengan dashboard web

Saat produk diupdate via dashboard Next.js (tidak melalui bot), cache bot tidak otomatis invalid. Perlu endpoint REST di bot (misal `POST /internal/cache/invalidate` dengan `KIOS_SERVICE_SECRET`) yang dipanggil dashboard setelah mutasi.

Alternatif: cache FAQ hanya untuk pertanyaan yang tidak bergantung data produk (jam buka, info kios) dan exclude pertanyaan harga → lebih konservatif tapi lebih aman.

### Q2: Apakah `kios:cache:faq:count` diperlukan untuk max entries?

Mengelola counter terpisah menambah kompleksitas. Alternatif: hapus `KIOS_FAQ_MAX_ENTRIES` dari scope awal, cukup andalkan TTL. Diskusikan dengan tim sebelum implementasi.

### Q3: Format jawaban yang di-cache vs yang dikembalikan ke user

Apakah kita simpan `response.Content` (mentah dari LLM) atau post-processed (setelah formatting Telegram Markdown)? Untuk Telegram, jawaban perlu Markdown. Jika disimpan mentah → saat di-serve dari cache harus di-apply format yang sama. Rekomendasi: simpan bentuk final yang sudah siap kirim.

### Q4: Bagaimana koordinasi Priority number dengan fitur #3, #4, #5?

Keempat fitur desain paralel. Perlu kesepakatan priority number sebelum implementasi:
- FAQ Cache: Priority 10
- Rate-limit per-user: Priority 20
- Counter harian: Priority 30
- Notif kuota: hanya di AfterLLM, Priority 50

Koordinasi via dokumen desain atau Slack/task sebelum coder mulai.

### Q5: Apakah hook BeforeLLM dapat publish ke bus secara aman?

`LLMHookRequest.Context` mengandung `TurnContext` dengan `ChatID` dan `Channel`. Tapi apakah `bus.PublishOutbound` thread-safe dari goroutine hook? Perlu verifikasi di `pkg/bus`. Jika tidak aman → fallback ke opsi lain (modify LLMResponse via synthetic response).

---

## Lampiran: Ringkasan Keputusan

| Keputusan | Pilihan | Alasan |
|-----------|---------|--------|
| Titik hook | `LLMInterceptor` di HookManager | Aditif 100%, 3 baris perubahan di agent_init.go |
| Hash | SHA256 8-hex | Cukup untuk N<1000, O(1) compute |
| Invalidasi | Generational versioning | O(1) tanpa SCAN, efisien untuk Upstash |
| TTL | 24 jam default | Sesuai perubahan harian kios desa |
| Collision handling | QuerySample comparison | Simple, aman, fallback ke LLM |
| Max entries | Counter terpisah (opsional) | Bisa diabaikan di v1 |
