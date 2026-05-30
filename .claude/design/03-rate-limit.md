# Design 03 — Rate-limit per-user + Debounce

**Subsistem:** Manajemen Kuota LLM (D7 Lapis 5)
**Tanggal:** 2026-05-30
**Status:** DRAFT — desain saja, belum ada kode

---

## 1. Tujuan & Ruang Lingkup

Mencegah satu pengguna Telegram menghabiskan kuota LLM bersama (Groq/Gemini)
dengan dua mekanisme berlapis:

1. **Rate-limit per-user** — tolak panggilan LLM ke-N+1 dalam satu jendela
   waktu; balas pesan ramah, jangan diam.
2. **Debounce** — kumpulkan pesan beruntun dari user yang sama dalam jendela
   pendek sebelum LLM dipanggil, mengurangi jumlah panggilan.

Cakupan yang TIDAK termasuk dalam dokumen ini:
- Cache FAQ (desain 02)
- Counter harian + mode hemat (desain 04)
- Notifikasi kuota ke owner (desain 05)

---

## 2. Data Model (Redis Keys)

### 2.1 Sliding-window rate-limit counter

```
kios:llm:ratelimit:{userID}:min   STRING   integer, TTL = 60s
kios:llm:ratelimit:{userID}:hour  STRING   integer, TTL = 3600s
```

- `{userID}` = Telegram sender ID (string), sudah tersedia di `msg.SenderID`
  dan di `LLMHookRequest.Meta.turnContext` (`chatID` Telegram = senderID untuk
  private chat).
- Dua key terpisah karena TTL berbeda; tidak ada kunci "window slot" — cukup
  INCR+EXPIRE (lihat §3.1).

### 2.2 Debounce buffer

```
kios:llm:debounce:{userID}   STRING   teks pesan terakhir, TTL = DEBOUNCE_WINDOW_MS / 1000 + 1s
```

- Nilai = gabungan pesan dalam buffer (dipisah newline).
- Saat buffer kadaluarsa (TTL habis) pesan dikirim ke LLM.
- Implementasi secara in-memory (lihat §3.2) lebih sederhana; Redis hanya
  fallback untuk multi-instance (Railway saat ini single instance, tapi desain
  harus siap).

### 2.3 Ringkasan namespace yang relevan (seluruh subsistem)

| Key pattern | Pemilik desain |
|---|---|
| `kios:cache:faq:*` | 02-cache-faq |
| `kios:llm:usage:{provider}:{YYYY-MM-DD}` | 04-daily-counter |
| `kios:llm:ratelimit:{userID}:*` | **dokumen ini** |
| `kios:llm:debounce:{userID}` | **dokumen ini** (opsional Redis) |

---

## 3. Algoritma

### 3.1 Sliding Window Rate-limit (pilihan vs Token Bucket)

**Pilihan: Sliding Window via INCR+EXPIRE**

Alasan memilih sliding window sederhana daripada token bucket:
- Kios desa: pola penggunaan sporadis, bukan streaming. Token bucket cocok
  untuk API publik dengan burst tinggi; sliding window lebih mudah dijelaskan
  dan di-debug.
- INCR+EXPIRE adalah operasi atomik di Redis — cukup tanpa Lua.
- Dua counter (per-menit, per-jam) memberikan proteksi dua lapis tanpa
  kompleksitas leaky-bucket.

**Atomicity — INCR+EXPIRE:**

```
n = INCR kios:llm:ratelimit:{userID}:min
if n == 1:
    EXPIRE kios:llm:ratelimit:{userID}:min 60
if n > KIOS_LLM_RPM_LIMIT:
    return RATE_LIMITED
```

Masalah atomicity: antara INCR dan EXPIRE ada celah kecil. Jika proses mati
setelah INCR tapi sebelum EXPIRE, key tidak pernah kadaluarsa. Solusi: gunakan
Lua script atau SET+NX untuk atomic set-with-expiry.

**Lua script yang direkomendasikan (atomic sliding counter):**

```lua
-- KEYS[1] = key counter, ARGV[1] = limit, ARGV[2] = TTL detik
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[2])
end
return n
```

Script dipanggil sekali; jika `n > limit` → tolak.

**Keunggulan vs Lua MULTI/EXEC:**
- Lua lebih portable ke Upstash Redis (REST mode mendukung EVAL).
- Upstash Redis SDK Go (`github.com/upstash/go-sdk`) mendukung `Do("EVAL", ...)`.

**Cek per-jam:** pola identik dengan TTL = 3600.

### 3.2 Debounce — Gabung Pesan Beruntun

**Tujuan:** jika user mengirim 3 pesan dalam 2 detik ("mau", "beli", "beras 2
kg"), gabungkan menjadi satu panggilan LLM dengan teks "mau\nbeli\nberas 2 kg".

**Mekanisme in-memory (pendekatan utama):**

```
type debounceEntry struct {
    userID    string
    buf       []string      // accumulated messages
    timer     *time.Timer   // fires after DEBOUNCE_WINDOW_MS
    mu        sync.Mutex
    flushFn   func(string)  // called with joined text
}

debounceMap sync.Map  // userID -> *debounceEntry
```

Alur:
1. Pesan masuk dari user X.
2. Jika ada entry aktif untuk X: tambahkan teks ke `buf`, reset timer.
3. Jika tidak ada: buat entry baru, mulai timer.
4. Saat timer tembak: gabungkan `buf`, panggil `flushFn(joined)`.
5. `flushFn` melanjutkan alur normal (cek cache → cek rate-limit → LLM).

**Trade-off latency:**
- Debounce menambah latensi = `DEBOUNCE_WINDOW_MS` (default 1500ms).
- User yang mengirim satu pesan lengkap menunggu 1,5 detik ekstra sebelum LLM
  mulai.
- Nilai 1500ms dipilih sebagai kompromi: cukup lama untuk menangkap burst
  "3 pesan dalam 1 detik", tapi tidak terasa lambat untuk pesan tunggal.
- Saat debounce aktif, pesan pertama TIDAK langsung diproses — ini berbeda
  dari "delay sambil nunggu lebih": selalu tunggu TTL penuh setelah pesan
  terakhir. Jika user terus mengetik tanpa jeda > DEBOUNCE_WINDOW_MS, timer
  terus direset dan LLM tidak pernah dipanggil. Ini risiko (lihat §7).

**Debounce dan slash command:**
- Slash command sudah ditangani SEBELUM debounce dan rate-limit (di
  `processMessage` → `handleCommand`, sebelum `runAgentLoop`). Tidak ada
  perubahan alur untuk slash command.

---

## 4. Titik Integrasi

### 4.1 Peta arsitektur pipeline

```
Telegram message
      |
      v
AgentLoop.processMessage()          [agent_message.go:123]
      |
      +-- handleCommand()           [slash commands, BYPASS semua filter]
      |
      v
[DEBOUNCE GATE]                     <-- titik hook baru (kios layer)
      |
      v
runAgentLoop()
      |
      v
Pipeline.CallLLM()
      |
      +-- p.Hooks.BeforeLLM()      [hooks.go:325] <-- titik hook utama
             |
             +-- [RATE-LIMIT CHECK] <-- LLMInterceptor baru
             |       |
             |       +--> jika limit: AbortTurn + balas pesan ramah
             |
             +-- [CACHE FAQ CHECK] (desain 02)
             |
             v
          LLM call
```

### 4.2 Hook eksplisit — `BeforeLLM` via `LLMInterceptor`

**File:** `pkg/agent/hooks.go` — interface `LLMInterceptor` (baris 79–82)
**Dipanggil oleh:** `Pipeline.CallLLM` di `pkg/agent/pipeline_llm.go` baris 82

```go
type LLMInterceptor interface {
    BeforeLLM(ctx context.Context, req *LLMHookRequest) (*LLMHookRequest, HookDecision, error)
    AfterLLM(ctx context.Context, resp *LLMHookResponse) (*LLMHookResponse, HookDecision, error)
}
```

**Struct baru yang diimplementasikan:** `KiosRateLimitHook` di
`pkg/tools/kios/ratelimit_hook.go` (file baru, < 200 baris).

`KiosRateLimitHook` mengimplementasikan `LLMInterceptor`:
- `BeforeLLM`: cek sliding window counter di Redis. Jika melebihi batas →
  kembalikan `HookDecision{Action: HookActionAbortTurn}` dan simpan pesan
  balasan di field metadata (lihat §4.4).
- `AfterLLM`: no-op, return `HookActionContinue`.

**Registrasi hook** di `agent_init.go` fungsi `NewAgentLoop` (setelah baris 111
`al.hooks = NewHookManager(...)`):

```go
if kiosStore != nil {
    rl := kios.NewRateLimitHook(kiosStore)
    al.hooks.Mount(agent.NamedHook("kios-ratelimit", rl))
}
```

### 4.3 Cara menyampaikan pesan balasan saat AbortTurn

`HookActionAbortTurn` menghentikan turn tapi tidak secara otomatis mengirim
pesan ke user. Dua opsi:

**Opsi A (direkomendasikan):** Sebelum return `AbortTurn`, hook memanggil
`al.bus.PublishOutbound(...)` secara langsung — ini pola yang sudah dipakai
`pipeline_llm.go` baris 388 untuk pesan "Context window exceeded".

Masalah: `KiosRateLimitHook` tidak punya referensi ke `al.bus`. Solusi: inject
`bus.MessageBus` ke hook saat konstruksi.

**Opsi B:** `HookActionAbortTurn` + set `LLMHookRequest.Messages` menjadi dummy
reply, lalu pipa finalize mengirimnya. Lebih rumit karena memerlukan modifikasi
finalize.

**Pilihan: Opsi A.** Hook menerima `*bus.MessageBus` dan `channel`/`chatID`
dari `LLMHookRequest.Meta.turnContext` (field `Channel`, `ChatID` tersedia via
`cloneTurnContext`). Hook publish langsung ke bus sebelum abort.

### 4.4 Identitas user di dalam hook

`LLMHookRequest.Meta` berisi `TurnContext`. Dari kode `turn_state.go`:
```go
ts.chatID      string   // Telegram chat ID (= senderID untuk private chat)
ts.channel     string   // "telegram"
```

`LLMHookRequest.Context` adalah `*TurnContext` (cloneTurnContext). Field yang
tersedia: perlu cek `turn_context.go` untuk field eksak, tapi dari
`turnState.eventMeta()` terlihat `SessionKey`, `TurnID`, `AgentID` tersedia.
`chatID` tersedia via `ts.chatID` yang dipasang ke Meta.

Fallback identitas: gunakan `Meta.SessionKey` yang sudah berisi channel + chatID
sebagai key unik per pengguna (format `telegram:{chatID}` atau `{agentID}:{chatID}`).

### 4.5 Debounce gate — titik integrasi

Debounce diimplementasikan **sebelum** `runAgentLoop`, yaitu di dalam
`processMessage` di `agent_message.go` (atau wrapper kios-specific).

Masalah: `processMessage` adalah metode core picoclaw, tidak boleh dimodifikasi.

**Solusi aditif:** Implementasikan debounce di lapisan Telegram channel handler,
sebelum pesan masuk ke `al.processMessage()`. Pada proyek ini channel Telegram
diinisialisasi di `cmd/` atau `pkg/channels/` — perlu dicek.

Alternatif yang lebih bersih: buat wrapper `KiosMessageDebouncer` yang
membungkus `bus.MessageBus`. Saat pesan masuk:
1. Cek apakah slash command → teruskan langsung ke bus tanpa debounce.
2. Bukan slash command → masukkan ke debounce map; saat timer tembak baru
   kirim ke bus.

`KiosMessageDebouncer` duduk di antara channel Telegram dan `MessageBus`. Ini
sepenuhnya aditif dan tidak menyentuh core picoclaw.

**File baru:** `pkg/tools/kios/debounce.go` (< 150 baris)

### 4.6 Urutan komposisi dengan subsistem lain

```
Pesan masuk Telegram
  1. [Debounce gate]            -- kios/debounce.go
  2. handleCommand()            -- slash command bypass LLM sepenuhnya
  3. BeforeLLM hook: cache FAQ  -- kios/cache_hook.go (desain 02)
     └─ cache HIT?  → AbortTurn + kirim cached reply
  4. BeforeLLM hook: rate-limit -- kios/ratelimit_hook.go (dokumen ini)
     └─ over limit? → AbortTurn + kirim pesan ramah
  5. BeforeLLM hook: mode hemat -- kios/savings_hook.go (desain 04)
     └─ kuota harian habis? → AbortTurn
  6. LLM call → provider
  7. AfterLLM: usage counter    -- kios/usage_hook.go (desain 04)
```

**Mengapa cache dicek SEBELUM rate-limit:**
- Cache hit = 0 token, 0 panggilan LLM → tidak menambah counter rate-limit.
- Rate-limit seharusnya hanya menghitung panggilan LLM nyata, bukan cache hit.
- Jika urutan dibalik (rate-limit dulu), user yang bertanya hal yang sama
  berkali-kali akan kena limit padahal tidak ada biaya token.

**Urutan hook di HookManager:** dikendalikan oleh field `Priority` di
`HookRegistration`. Nilai lebih kecil = dieksekusi lebih dulu.

```go
// Contoh priority
al.hooks.Mount(HookRegistration{Name: "kios-cache",     Priority: 10, Hook: cacheHook})
al.hooks.Mount(HookRegistration{Name: "kios-ratelimit", Priority: 20, Hook: rateLimitHook})
al.hooks.Mount(HookRegistration{Name: "kios-savings",   Priority: 30, Hook: savingsHook})
```

---

## 5. Environment Variables & Config Baru

| Variabel | Default | Keterangan |
|---|---|---|
| `KIOS_LLM_RPM_LIMIT` | `5` | Maks panggilan LLM per menit per user |
| `KIOS_LLM_RPH_LIMIT` | `30` | Maks panggilan LLM per jam per user |
| `KIOS_LLM_RATELIMIT_ENABLED` | `true` | Nonaktifkan dengan `false` |
| `KIOS_DEBOUNCE_MS` | `1500` | Jendela debounce dalam milidetik |
| `KIOS_DEBOUNCE_ENABLED` | `true` | Nonaktifkan dengan `false` |
| `KIOS_OWNER_IDS` | — | Sudah ada; owner di-whitelist dari rate-limit |

**Parsing config:** dibaca sekali saat `NewRateLimitHook()` dan
`NewKiosMessageDebouncer()` dipanggil di `agent_init.go`. Tidak ada config
file baru — konsisten dengan pola env-only proyek ini.

**Whitelist owner:** cek `strings.Contains(KIOS_OWNER_IDS, userID)` di awal
`BeforeLLM`. Jika owner → skip counter, return `HookActionContinue`.

---

## 6. Rencana Test

Semua test di `pkg/tools/kios/ratelimit_hook_test.go` dan `debounce_test.go`,
table-driven, menggunakan `miniredis` untuk mock Redis.

### 6.1 Rate-limit tests

| Nama test | Skenario | Expected |
|---|---|---|
| `TestRateLimit_BelowLimit` | 3 panggilan, limit=5/menit | semua lolos |
| `TestRateLimit_AtLimit` | 5 panggilan, limit=5/menit | semua lolos |
| `TestRateLimit_OverLimitMinute` | 6 panggilan, limit=5/menit | ke-6 ditolak |
| `TestRateLimit_ResetAfterWindow` | 5 panggilan, mundurkan jam 61 detik, 1 panggilan lagi | lolos |
| `TestRateLimit_PerHourLimit` | 30 panggilan dalam menit berbeda, limit=30/jam | ke-31 ditolak |
| `TestRateLimit_OwnerBypass` | 10 panggilan dari owner ID | semua lolos |
| `TestRateLimit_Disabled` | env ENABLED=false, 100 panggilan | semua lolos |
| `TestRateLimit_MultiUser` | user A 5x, user B 5x, limit=5 | A dan B masing-masing 5 lolos |
| `TestRateLimit_LuaAtomic` | concurrency 10 goroutine, limit=5 | tepat 5 lolos |

### 6.2 Debounce tests

| Nama test | Skenario | Expected |
|---|---|---|
| `TestDebounce_SingleMessage` | 1 pesan, tunggu TTL | 1 flush dipanggil |
| `TestDebounce_BurstMessages` | 3 pesan dalam 200ms, window=1500ms | 1 flush dengan 3 pesan digabung |
| `TestDebounce_ResetTimer` | pesan ke-2 sebelum window habis | timer direset |
| `TestDebounce_SlashCommandBypass` | pesan "/jual beras 2" | langsung forward, no debounce |
| `TestDebounce_Disabled` | env ENABLED=false | langsung forward |
| `TestDebounce_TwoUsers` | user A dan B bersamaan | dua buffer terpisah |
| `TestDebounce_LongTyping` | pesan setiap 1400ms selama 10 detik | tidak pernah flush selama mengetik |

### 6.3 Integration test

`TestRateLimitHook_AbortTurn`: buat `LLMHookRequest` palsu dengan userID yang
sudah habis counternya, pastikan `BeforeLLM` mengembalikan `AbortTurn` dan
pesan balasan dikirim ke bus mock.

---

## 7. Edge Cases & Risiko

### 7.1 Clock skew di Upstash Redis

INCR+EXPIRE menggunakan clock server Redis, bukan clock bot. Aman dari skew
antar-instance bot. Risiko: jika Upstash punya clock drift antar shard
(cluster mode), tapi Upstash free tier adalah single primary — tidak relevan.

### 7.2 Debounce infinite loop (user selalu mengetik)

Jika user mengirim pesan setiap 1 detik dengan window 1,5 detik, timer selalu
direset dan LLM tidak pernah dipanggil. User tidak mendapat respons.

**Mitigasi:** tambahkan batas maksimum buffer — jika buffer sudah berisi N pesan
(mis. 5) atau total teks > 1000 karakter, flush paksa meski timer belum habis.
Ini mencegah "starvation" tapi menambah kompleksitas implementasi.

Alternatif: gunakan "debounce dengan max-wait" — timer direset setiap pesan baru,
tapi ada hard deadline (mis. 5 detik dari pesan pertama). Setelah 5 detik,
flush tanpa menunggu jeda.

### 7.3 Burst storm saat kios ramai (jam 07.00 pagi)

Banyak user bertanya bersamaan → banyak counter INCR bersamaan di Redis.
Upstash free tier punya batas throughput operasi. Jika INCR gagal (timeout
Redis), fail-open (izinkan panggilan LLM) atau fail-closed (tolak)?

**Rekomendasi: fail-open** dengan log warning. Lebih baik sesekali melewati
limit daripada semua user tiba-tiba tidak dapat respons saat Redis lambat.

### 7.4 Owner bypass dan privilege escalation

`KIOS_OWNER_IDS` diisi dari env, tidak bisa diubah runtime. Aman dari user
yang mencoba "menjadi owner". Tapi jika owner mengirim spam, tidak ada
proteksi. Tradeoff yang disengaja (owner bertanggung jawab atas kuotanya).

### 7.5 userID spoofing

Telegram senderID dari long-polling sudah divalidasi oleh Telegram API.
Tidak ada risiko spoofing di lapisan kios.

### 7.6 Debounce + AbortTurn race condition

Jika debounce flush terjadi bersamaan dengan `HookActionAbortTurn` dari hook
lain (mis. cache hit), debounce mengirim pesan ke bus, tapi turn sudah aborted.
Pesan masuk ke bus sebagai pesan baru di turn berikutnya — tidak merusak state,
tapi bisa menyebabkan double-response kecil.

**Mitigasi:** pastikan debounce flush hanya memforward pesan ke bus (bukan
langsung ke pipeline); pipeline yang menangani collision.

### 7.7 Multiple Bot Instances (masa depan)

Saat ini Railway menjalankan 1 instance. Jika di-scale ke 2 instance,
debounce in-memory tidak shared antar instance. Rate-limit Redis tetap akurat
karena atomic. Debounce perlu dimigrasi ke Redis-backed jika multi-instance
diperlukan.

---

## 8. Pertanyaan Terbuka

1. **TurnContext field eksak:** Field `chatID`/`senderID` perlu dikonfirmasi
   dari `turn_context.go` (belum dibaca). Hook perlu field ini untuk key Redis.
   Apakah `LLMHookRequest.Meta.SessionKey` sudah berisi senderID Telegram, atau
   perlu field tambahan di `HookMeta`?

2. **Debounce placement:** Wrapper `KiosMessageDebouncer` di bus atau di channel
   Telegram handler? Perlu cek `pkg/channels/telegram/` untuk titik injeksi yang
   paling bersih. Jika channel tidak expose hook, debounce harus di level bus.

3. **Pesan balasan saat AbortTurn:** Opsi A (hook publish ke bus langsung)
   memerlukan inject `*bus.MessageBus` ke hook. Apakah ada pola lebih bersih
   (mis. field `ReplyContent` di `HookDecision`) yang bisa ditambahkan tanpa
   modifikasi besar ke core hooks.go?

4. **Nilai default RPM/RPH:** `5/menit` dan `30/jam` adalah estimasi awal.
   Perlu data penggunaan nyata kios untuk kalibrasi. Terlalu ketat = user
   frustrasi; terlalu longgar = kuota habis. Apakah ada usage log yang bisa
   dipakai?

5. **Komposisi dengan desain 04 (mode hemat):** Saat mode hemat aktif, semua
   LLM call diblokir (bukan per-user). Rate-limit per-user tidak relevan di
   mode ini. Apakah rate-limit hook harus sadar mode hemat, atau biarkan mode
   hemat hook yang intercept lebih dulu (via priority lebih rendah)?

6. **Lua EVAL di Upstash Redis:** Upstash REST mode mendukung EVAL, tapi
   `github.com/redis/go-redis/v9` (yang dipakai store.go) berkomunikasi via
   TCP `rediss://`. Perlu konfirmasi Lua EVAL bisa dipakai atau harus pakai
   pipelined INCR+EXPIRE (risiko non-atomic kecil tapi ada).
