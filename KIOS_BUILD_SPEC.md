# Kios-PicoClaw — Build Spec (single source of truth)

> Goal: rewrite the "Kios Openclaw" village-shop AI assistant as **native Go tools on top of picoclaw**,
> deployable to **Railway** (free/trial tier). Data persists in **Upstash Redis** (Railway FS is ephemeral).
> Scope of THIS build: **core tools only** — `stok`, `kasir`, `laporan`, `harga` — plus Telegram + Groq/Gemini
> + Bahasa-Indonesia SKILL.md + Dockerfile + Railway config. Secondary skills (supplier, promo, market-intel,
> self-learner, etc.) are explicitly OUT of scope for now.

Repo: this directory (`~/kios-picoclaw`) is a clone of `sipeed/picoclaw`. We ADD a kios package + register it.

## Toolchain (CRITICAL — every Go command must export PATH first)
Go is at `~/sdk/go` and NOT on the non-interactive PATH. Prefix every build/test command:
```bash
export GOROOT=$HOME/sdk/go PATH=$HOME/sdk/go/bin:$PATH GOPATH=$HOME/go
```
Build: `cd ~/kios-picoclaw && make build` (verified working, produces `build/picoclaw`).
Test:  `go test ./pkg/tools/kios/...`

## PicoClaw facts (verified from source)
- **Tool interface** (`pkg/tools/shared/base.go`, package `toolshared`):
  ```go
  type Tool interface {
      Name() string
      Description() string
      Parameters() map[string]any   // JSON-schema object: {"type":"object","properties":{...},"required":[...]}
      Execute(ctx context.Context, args map[string]any) *ToolResult
  }
  ```
  Look at `pkg/tools/cron.go` for a full reference implementation of a tool (Name/Description/Parameters/Execute).
  Find the concrete `*ToolResult` type + helper constructors in `pkg/tools/shared/result.go` and use them.
- **Registry**: `tools.NewToolRegistry()` built in `pkg/agent/instance.go:94`; tools registered via
  `agent.Tools.Register(tool)`. Core tools are wired in `pkg/agent/agent_init.go`. **Inject kios tools here**
  (a dedicated block, ideally gated by a config flag like `config.Tools.Kios.Enabled`, default true).
  `pkg/agent/agent_inject.go` shows an injection loop pattern.
- **Config**: `config/config.example.json` (version 3). Keys we use:
  - `model_list[]`: each `{model_name, model, api_keys[], api_base}`.
    - Groq (primary): `{"model_name":"groq-llama","model":"openai/meta-llama/llama-4-scout-17b-16e-instruct","api_keys":["$GROQ_API_KEY"],"api_base":"https://api.groq.com/openai/v1"}` (Groq is OpenAI-compatible).
    - Gemini (fallback): `{"model_name":"gemini","model":"gemini/gemini-2.0-flash","api_keys":["$GEMINI_API_KEY"]}` (confirm exact provider prefix against `pkg/providers`).
  - `agents.defaults.model_name` → set to the Groq model_name.
  - `channel_list.telegram`: `{enabled:true, type:"telegram", allow_from:["<user_ids>"], settings:{token:"$TELEGRAM_BOT_TOKEN"}}`.
    `allow_from` IS the whitelist (replaces TELEGRAM_WHITELIST).
- **Telegram** channel is native: `pkg/channels/telegram`. Long-poll, no inbound port needed.
- **Gateway/health**: serves health on `:18790` (see docker/Dockerfile HEALTHCHECK). For Railway, bind to `$PORT` if set.
- **SKILL.md**: picoclaw loads behavioral skills hierarchically from `SKILL.md` files in the workspace
  (`~/.picoclaw/workspace`, configurable). Investigate `pkg/skills/loader.go` for the exact path/format.

## Redis data model (Upstash, via github.com/redis/go-redis/v9, rediss:// URL from $UPSTASH_REDIS_URL)
Replace the CSV/JSON files. Suggested keys (architect may refine):
- `kios:produk` — HASH, field = product id (e.g. "002"), value = JSON of product.
- `kios:transaksi` — LIST, RPUSH JSON per sale. `kios:seq:trx` — INCR counter → TRX-0001.
- `kios:pembelian` — LIST + `kios:seq:pem` → PEM-0001.
- `kios:price_history` — LIST + `kios:seq:phg` → PHG-0001.
- `kios:shift` — STRING(JSON) current shift state.
- `kios:users` — HASH, field = phone, value = JSON {phone,nama,role,aktif} for RBAC.
Provide a one-time **seed/migration** path: import existing CSVs from the old project on first run if the
Redis keys are empty (old data lives in `~/kios-openclaw/data/`). Keep it idempotent.

## Data schemas (from old project — exact fields)
- **produk (stok.csv)**: id, nama, kategori, satuan, stok, harga_beli, harga_jual, stok_minimum, stok_kritis, supplier, last_update, has_exp, exp_date
- **transaksi.csv**: id, tanggal, jam, produk_id, nama_produk, kategori, qty, harga_satuan, total, metode_bayar, kasir, catatan, session_id
- **pembelian.csv**: id, session_id, tanggal, jam, produk_id, nama_produk, qty, harga_beli, subtotal, supplier, kasir, catatan
- **price-history.csv**: id, tanggal, jam, produk_id, nama_produk, harga_lama, harga_baru, selisih, supplier, kasir
- **users.json**: phone → {phone, nama, role ("kasir"|"owner"), aktif (bool), ditambahkan}

## The 4 tools to implement (in `pkg/tools/kios/`)
Each = one picoclaw Tool with an `action` enum param + per-action params. Mirror the old Python/JS logic.
Product lookup `cari_produk`: match by exact id, OR substring of nama, OR all query words present in nama (case-insensitive).

### tool `kios_stok`
actions:
- `cek` → list all products.
- `cari` {produk} → find one product.
- `jual` {produk, qty, metode=tunai} → validate qty>0 & stok>=qty; decrement stok; append transaksi (TRX-####, total=qty*harga_jual, kasir from session/role); return {item,qty,total,sisa,txId}. Emit low-stock signal when sisa<=stok_kritis or <=0 (a log/notif is fine).
- `tambah` {produk, qty, harga(beli), supplier, auto_create=false} → restock; if product missing & auto_create → create (margin 15% default harga_jual=int(harga_beli*1.15), id zero-padded 3 digits); log price change to price_history if harga_beli changed; append pembelian (PEM-####).
- `tambah_produk` {nama, kategori=umum, satuan=pcs, stok, harga_beli, harga_jual, stok_minimum=5, stok_kritis=2, supplier, exp_date} → new product; nama & harga_jual required.
- `hapus` {produk}, `set_stok` {produk, stok_baru}, `update_exp` {produk, exp_date},
- `batalkan_tx` {id} → restore stock + remove transaksi,
- `stok_menipis` → products where stok<=stok_minimum (+ qty_dibutuhkan = max(0, stok_minimum*3 - stok)).

### tool `kios_kasir`
- `jual` {produk, qty, metode=tunai, bayar} → calls stok-jual logic + returns a formatted *struk* (receipt) string (header "🧾 STRUK KIOS CERDAS", lokasi "Rote Barat Laut, Rote Ndao", items, total, bayar, kembalian, #txId + WITA time = UTC+8). hitung kembalian; error if bayar<total.
- `buka_shift` {kasir, saldo_awal} / `tutup_shift` {saldo_akhir} / `status_shift` — single open shift at a time; compute omzet from transaksi since waktu_buka.

### tool `kios_laporan`
- `ringkas` {tanggal?} daily, `mingguan`, `bulanan`, `laba` {periode: hari_ini|minggu|bulan}, `riwayat` {periode} (last 20), `terlaris` {periode, top=10}, `riwayat_harga` {produk?}.
- laba = omzet(sum total) - modal(sum qty*harga_beli per produk). top3 by qty. stokKritis = nama where stok<=stok_kritis.

### tool `kios_harga`
- `cek` {produk}, `update` {produk, harga_jual, harga_beli?} (log to price_history), `estimasi` {produk, harga_beli_baru?} (margins 10/15/20/25% + optional market ref), `prediksi` {produk} (trend from price_history, needs >=2 points).

## SKILL.md (Bahasa Indonesia)
A workspace skill that gives the agent: persona (asisten kios desa, ramah, Bahasa Indonesia, panggil "kak"),
when to call each kios tool, RBAC rules (only role kasir/owner may jual/tambah/update harga; owner-only for
hapus/batalkan_tx/tambah_produk/set_stok), and to format rupiah as "Rp 15.000". Sender identity comes from the
Telegram user; map to `kios:users` for role. If user not in users or aktif=false → refuse politely.

## Deploy (Railway)
- `config.json` reads secrets from env (TELEGRAM_BOT_TOKEN, GROQ_API_KEY, GEMINI_API_KEY, UPSTASH_REDIS_URL, KIOS_ALLOW_FROM).
- `.env.example` documents all required vars.
- Adapt `docker/Dockerfile` (already a clean multi-stage Go→alpine build). Add `railway.json` (or `railway.toml`)
  with the Dockerfile builder + start command `picoclaw gateway` (confirm the run subcommand).
- Bind health to `$PORT` when present (Railway sets $PORT).
- Document exact Railway deploy steps in a short DEPLOY-RAILWAY.md.

## Constraints
- Keep every file < 500 lines. Validate inputs at boundaries. NEVER commit secrets / .env.
- Match upstream Go style; run `gofmt`. Add unit tests for the kios tools (table-driven, use a Redis mock or miniredis).
- Do NOT break the existing picoclaw build — kios tools are additive.
