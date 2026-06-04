# Laporan Bug & Error — Smoke Test Dashboard Kios

> **Tanggal:** 2026-05-31
> **Lingkup:** `kios-dashboard/` (Next.js 15) — smoke test runtime sesuai `dokument-panduan.md` §4
> **Catatan:** Pemeriksaan **read-only**. Tidak ada file sumber yang diubah. File ini
> dibuat khusus sebagai dokumentasi temuan; bukan bagian dari kode aplikasi.

---

## Metodologi

| Jenis | Perintah / Alat | Hasil |
|-------|-----------------|-------|
| Static check | `npm run typecheck` (tsc --noEmit) | ✅ Lulus, 0 error |
| Runtime smoke test | `npm install` + `npm run dev` → uji halaman via **agent-browser** + `curl` | Lihat temuan |
| Dependency audit | `npm audit` | 2 medium |
| Security scan | **ruflo** `security scan --depth quick` | 2 medium (konsisten dg npm audit) |

Verifikasi kebersihan: `git status` hanya menampilkan file untracked (`dokument-panduan.md`,
`LAPORAN-BUG-SMOKETEST.md`). `package-lock.json` dikembalikan; `node_modules` ter-gitignore.

---

## 🐛 BUG-001 [MEDIUM] — Ikon produk `.svg` di storefront publik kena auth gate

**File:** `kios-dashboard/src/middleware.ts:52`

**Deskripsi:**
Matcher middleware hanya mengecualikan file `.png`, tidak `.svg`:

```ts
matcher: ["/((?!_next/static|_next/image|favicon.ico|icon.svg|robots.txt|.*\\.png$).*)"],
```

`src/lib/produk-image.ts` (`categoryImage()`) menghasilkan path fallback gambar produk
`/produk/<kategori>.svg` (file tersedia di `public/produk/`: gas, mie, sembako, rokok,
minuman, kebutuhan, snack, umum), dipakai di `src/components/toko/storefront-view.tsx`.

**Dampak:**
Halaman **publik** `/toko` dan `/mall` (didesain tanpa login) — setiap request ikon SVG
di-redirect **307 → `/login`** dan mengembalikan HTML alih-alih gambar. Fallback gambar
produk **rusak** bagi pembeli yang tidak login.

**Langkah reproduksi:**
1. `npm run dev`
2. `curl -i http://localhost:3000/produk/mie.svg`

**Bukti aktual:**
```
GET /produk/mie.svg     → 307 → /login?next=%2Fproduk%2Fmie.svg
Content-Type: text/html; charset=utf-8        (bukan image/svg+xml)
```
Pembanding `GET /produk/x.png` → 404 (tidak di-redirect, karena `.png` dikecualikan).
Log dev server penuh dengan baris:
```
GET /login?next=%2Fproduk%2Fgas.svg 200
GET /login?next=%2Fproduk%2Fmie.svg 200
GET /login?next=%2Fproduk%2Fsembako.svg 200
... (rokok, minuman, kebutuhan)
```

**Saran perbaikan (belum diterapkan):**
Tambahkan `.svg` ke daftar pengecualian matcher, atau kecualikan prefix `/produk/`.
Contoh: `...|.*\\.png$|.*\\.svg$).*)`.

---

## ⚠️ SEC-001 [MEDIUM] — 2 CVE dependency (dev tooling)

Dikonfirmasi dua alat (`npm audit` **dan** ruflo `security scan`): **2 medium, 0 critical, 0 high.**

| Paket | Severity | Isu |
|-------|----------|-----|
| `postcss` <8.5.10 | MEDIUM | XSS via unescaped `</style>` — GHSA-qx2v-qp2m-jg93 |
| `next` (transitif) | MEDIUM | Bergantung pada versi postcss rentan |

**Dampak:** terbatas; keduanya transitif/dev-tooling.
**Catatan:** `npm audit fix --force` akan menurunkan Next.js (breaking change) — **jangan**
dijalankan tanpa pertimbangan. ruflo `security cve --check` tidak punya DB terintegrasi
(merujuk balik ke `npm audit`), jadi tidak menambah info baru.

---

## ✅ Komponen yang BERFUNGSI NORMAL (tidak ada bug)

- **TypeScript typecheck**: lulus, 0 error.
- **Auth gate**: 10 rute admin (`/dashboard`, `/produk`, `/kasir`, `/laporan`, `/penjualan`,
  `/suplier`, `/pesanan`, `/pengguna`, `/impor`, `/pengaturan`) semua redirect **307 → `/login?next=...`**
  saat belum login (perilaku benar).
- **Login**: kode salah → tetap di `/login`, pesan benar ("Hanya pemilik dan kasir terdaftar yang dapat masuk").
- **Storefront `/toko` & `/mall`**: render publik tanpa auth; produk tampil dari Upstash Redis
  (Beras Medium 5kg, Gas LPG 3kg, Gula Pasir 1kg, dll.) → koneksi REST OK.
- **API**: `/api/health` → 200; `/api/summary`, `/api/toko`, `/api/orders` → 307 (terlindungi, sesuai harapan).
- **Console browser**: tidak ada error/warning JS di `/login`, `/toko`, `/mall`, `/dashboard`
  (hanya info React DevTools & Fast Refresh).
- **Konfigurasi `.env`**: 7 key terisi semua (sesuai checklist `dokument-panduan.md` §6).

---

## Ringkasan

| ID | Severity | Status | Komponen |
|----|----------|--------|----------|
| BUG-001 | MEDIUM | Terbuka | `src/middleware.ts` — SVG storefront |
| SEC-001 | MEDIUM | Terbuka | dependency `postcss`/`next` |

**Total:** 1 bug kode, 1 isu dependency. **Tidak ada** error runtime/crash; tidak ada critical/high.

---

# Bagian 2 — Smoke Test Bot Telegram (live)

> **Tanggal:** 2026-05-31 ~20:50–20:55 WITA
> **Cara:** Bot dijalankan **lokal** (binary `build/picoclaw gateway`, config dirender ke
> `.picoclaw-dev/` di luar tracking git — TIDAK mengubah file repo). User mematikan Railway
> sementara agar polling diambil instance lokal. Chat lewat **Telegram Web** (Chrome DevTools
> MCP) memakai akun owner (YourJackJack, ID 8753360239).
> **Bot:** @picoclawOmbok_ai_bot ("Kios Cerdas"). 28 tools, 8/8 skills, health 200.

## Hasil command (semua ✅ PASS)

| Perintah | Hasil |
|----------|-------|
| `/stok` | Daftar 10 produk + status stok (Gas LPG 3kg 🔴 menipis) |
| `/harga Beras` | Harga jual/modal/margin/stok Beras Medium 5kg |
| `/laporan` | Laporan hari ini (0 transaksi, shift belum buka) |
| `/menipis` | 1 item menipis (Gas LPG 3kg, saran restock ±10) |
| `/promo` | Tidak ada promo aktif |
| `/pasar` | Perbandingan harga kios vs pasar |
| `/suplier` | 2 supplier terdaftar |
| `/shift` | Status shift (belum dibuka) |
| Chat AI "berapa stok beras?" | AI panggil tool & jawab "40 karung" natural ✅ |

## Test tulis + rollback (✅ berhasil, data dipulihkan)

1. `/jual Kopi Sachet 1` → struk **TRX-0001**, stok Kopi Sachet 200 → **199** ✅
2. `tolong batalkan transaksi TRX-0001` (via AI, hak owner) → dibatalkan, stok 199 → **200** ✅
3. Verifikasi `/stok` → Kopi Sachet **200 pcs** (kembali ke awal). **Tidak ada data tersisa berubah.**

## Temuan Bagian 2

### CATATAN-001 [INFO, bukan bug kode] — Error 429 "quota exceeded" di riwayat chat
Riwayat chat (30 Mei, 21:07 & 21:13) menampilkan `Error processing message: LLM call failed...
Status: 429 ... You exceeded your current quota`. Saat diuji ulang dengan key di `deploy/.env`,
AI **berfungsi normal** (tidak 429). → Itu **habis kuota LLM** pada deployment/ key lama,
**bukan bug kode**. Pastikan key Groq/Gemini produksi punya kuota.

### BUG-002 [LOW/UX] — Tidak ada slash command untuk batal transaksi
`/batal TRX-0001` → "Perintah tidak dikenal: /batal". Pembatalan hanya bisa via chat AI
("batalkan transaksi TRX-0001") atau dashboard. Untuk kios dengan sinyal/kuota AI terbatas,
ini celah UX — owner tak bisa batal transaksi tanpa AI. Saran: tambah slash command
`/batal <TRX-id>` (rule-based, 0 token). *(Tidak diterapkan — hanya laporan.)*

### OPS-001 [LOW/deploy] — entrypoint.sh tidak bisa jalan mentah di lokal
`deploy/entrypoint.sh` hardcode path Docker `/app/workspace` (baris 18) → gagal saat
dijalankan langsung di host (`cp: /app/workspace/. tidak ada`). Wajar (didesain untuk
container), tapi menyulitkan test lokal. Untuk test lokal: pre-provision `workspace/` ke
`$PICOCLAW_HOME/workspace` lalu jalankan binary, atau jalankan via Docker. *(Bukan bug
produksi; catatan untuk dev.)*

## Ringkasan Bagian 2

| ID | Severity | Temuan |
|----|----------|--------|
| CATATAN-001 | INFO | 429 di riwayat = kuota LLM habis (key lama), bukan bug |
| BUG-002 | LOW/UX | Tak ada `/batal` slash command (batal hanya via AI/dashboard) |
| OPS-001 | LOW | entrypoint.sh hanya untuk Docker, gagal run lokal mentah |

**Inti:** Semua fungsi bot **berjalan normal** (9 perintah + AI + jual + batal). Operasi tulis
diuji & **di-rollback bersih**. Tidak ada error/crash pada kode bot.
