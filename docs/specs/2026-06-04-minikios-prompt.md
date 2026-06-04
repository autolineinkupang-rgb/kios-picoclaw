# PROMPT CLAUDE CODE — Mini Kios Desa
# Paste seluruh isi file ini ke Claude Code di terminal
# Jalankan di: ~/Publik/project/minikios

---

Buat project lengkap **Mini Kios Desa** dari nol di direktori `~/Publik/project/minikios`.

Project ini adalah sistem manajemen kios kecil di desa (Rote Ndao, NTT) yang menjual pulsa,
paket data, token listrik PLN, dan transfer dana. Dibangun di atas PicoClaw sebagai AI agent,
dengan Telegram sebagai channel utama, Railway sebagai host 24/7, dan Upstash Redis sebagai
database. Lokal hanya menjalankan dua UI: Dashboard Kios (transaksi harian) dan PicoClaw
Web UI (konfigurasi sesekali).

---

## ARSITEKTUR SISTEM

```
Telegram Bot
    │
    ▼ (long polling, 24/7)
PicoClaw Agent ──── Railway Cloud
    │     │
    │     ├── AI Models: Groq (utama) → Gemini (fallback)
    │     └── GitHub Repo → auto-deploy Railway
    │
    ▼
Upstash Redis (database utama, sync, backup)
    │
    ├── Dashboard Kios (lokal, React+Vite, transaksi harian)
    └── PicoClaw Web UI (lokal, port 18800, konfigurasi sesekali)
```

**Prinsip utama:**
- TIDAK ada gateway PicoClaw yang jalan di lokal
- TIDAK menggunakan Oracle Cloud
- Railway = satu-satunya runtime agent (Docker, free tier)
- Upstash Redis = satu-satunya database (bukan file, bukan SQLite)
- Dashboard lokal konek langsung ke Upstash Redis via REST API
- Data tidak pernah hilang karena semua di Redis, bukan di memori lokal

---

## LANGKAH 1 — SETUP DIREKTORI & GITHUB

```bash
mkdir -p ~/Publik/project/minikios
cd ~/Publik/project/minikios
git init
```

Buat repository baru di GitHub dengan nama `minikios-desa` (public atau private, sesuai
preferensi). Hubungkan:

```bash
git remote add origin https://github.com/USERNAME/minikios-desa.git
```

Struktur direktori lengkap yang harus dibuat:

```
minikios-desa/
├── agent/                        ← PicoClaw agent (di-deploy ke Railway)
│   ├── AGENT.md                  ← persona & aturan asisten
│   ├── config/
│   │   └── kios.json             ← konfigurasi model, channel, tools
│   ├── workspace/
│   │   └── harga.md              ← daftar harga (edit untuk update harga)
│   ├── scripts/
│   │   └── entrypoint.sh         ← entrypoint Railway
│   ├── Dockerfile                ← untuk Railway deploy
│   └── .env.example              ← template environment variables
│
├── dashboard/                    ← Dashboard Kios lokal (React + Vite)
│   ├── src/
│   │   ├── main.jsx
│   │   ├── App.jsx
│   │   ├── pages/
│   │   │   ├── Transaksi.jsx     ← halaman utama kasir
│   │   │   ├── Pelanggan.jsx
│   │   │   ├── Utang.jsx
│   │   │   ├── Stok.jsx
│   │   │   ├── Supplier.jsx
│   │   │   ├── Rekap.jsx
│   │   │   ├── Users.jsx
│   │   │   └── Backup.jsx
│   │   ├── components/
│   │   │   ├── Sidebar.jsx
│   │   │   ├── Header.jsx
│   │   │   ├── TransaksiForm.jsx
│   │   │   ├── UtangCard.jsx
│   │   │   ├── StokTable.jsx
│   │   │   └── RekapHarian.jsx
│   │   ├── lib/
│   │   │   ├── redis.js          ← Upstash Redis REST client
│   │   │   ├── auth.js           ← login admin/kasir
│   │   │   └── format.js        ← format Rupiah, tanggal, WITA
│   │   └── styles/
│   │       └── kios.css          ← styling dashboard
│   ├── index.html
│   ├── vite.config.js
│   └── package.json
│
├── docs/
│   ├── SETUP.md                  ← panduan setup lengkap
│   ├── DEPLOY_RAILWAY.md         ← cara deploy ke Railway
│   └── CARA_PAKAI.md             ← panduan kasir sehari-hari
│
├── .github/
│   └── workflows/
│       └── deploy.yml            ← CI/CD: push main → Railway deploy
│
├── .gitignore
├── README.md
├── AGENT.md                      ← referensi dari project lama
├── HOWITWORK.md
└── ROADMAP.md
```

---

## LANGKAH 2 — AGENT PICOCLAW (Railway)

### agent/AGENT.md

Tulis ulang AGENT.md yang lebih lengkap dari versi lama. Agent harus:

1. **Cek harga** — baca `workspace/harga.md`, tidak mengarang harga
2. **Catat transaksi** — simpan ke Upstash Redis dengan format:
   `transaksi:{timestamp} → {produk, nominal, harga, admin, total, pelanggan, kasir, waktu_WITA}`
3. **Kelola utang/piturang** — catat, lihat, tandai lunas (pembeli DAN supplier)
4. **Cek stok** — baca stok dari Redis, ingatkan kalau menipis
5. **Data pelanggan** — simpan nama, nomor HP, riwayat transaksi
6. **Role-aware** — bedakan perintah dari admin vs kasir
7. **Fleksibel LLM** — pakai Groq untuk respons cepat, fallback ke Gemini kalau limit habis
8. **Bahasa** — Indonesia ringkas + sapaan ringan bahasa Rote kalau perlu

Aturan keras:
- Jangan mengarang harga, jangan simpan PIN/password
- Konfirmasi nomor & nominal sebelum catat
- Semua waktu pakai zona WITA (UTC+8)
- Hanya mencatat, tidak mengirim pulsa/uang sungguhan

### agent/workspace/harga.md

Buat file harga lengkap dengan format tabel Markdown:

```markdown
# Daftar Harga Kios — [Nama Kios]
Terakhir diupdate: [tanggal]

## Pulsa
| Nominal | Harga Jual | Admin |
|---------|-----------|-------|
| 5.000   | 6.500     | 0     |
| 10.000  | 11.500    | 0     |
| 20.000  | 21.500    | 0     |
| 25.000  | 26.500    | 0     |
| 50.000  | 51.500    | 0     |
| 100.000 | 102.000   | 0     |

## Paket Data
| Produk              | Harga Jual | Admin |
|--------------------|-----------|-------|
| Telkomsel 1GB/7hr  | 15.000    | 0     |
| Telkomsel 3GB/30hr | 35.000    | 0     |
| Telkomsel 10GB/30hr| 85.000    | 0     |
| XL 1.5GB/7hr       | 13.000    | 0     |
| XL 4GB/30hr        | 30.000    | 0     |
| Axis 1GB/30hr      | 10.000    | 0     |
| Indosat 2GB/7hr    | 14.000    | 0     |
| Indosat 7GB/30hr   | 45.000    | 0     |

## Token Listrik PLN
| Nominal  | Harga Jual | Admin |
|---------|-----------|-------|
| 20.000  | 21.500    | 1.500 |
| 50.000  | 51.500    | 1.500 |
| 100.000 | 101.500   | 1.500 |
| 200.000 | 201.500   | 1.500 |
| 500.000 | 501.500   | 1.500 |

## Transfer Dana
| Nominal        | Admin  |
|---------------|--------|
| < 100.000     | 5.000  |
| 100.000–500.000| 6.500 |
| > 500.000     | 8.000  |

## Stok Fisik (diupdate manual)
| Produk       | Stok | Stok Minimum |
|-------------|------|-------------|
| Kartu Perdana Telkomsel | 5 | 3 |
| Kartu Perdana XL       | 3 | 2 |
| Kartu Perdana Axis     | 5 | 2 |
```

### agent/config/kios.json

```json
{
  "agent_file": "AGENT.md",
  "workspace": "workspace/",
  "model_list": [
    {
      "name": "groq-llama",
      "provider": "groq",
      "model": "llama-3.3-70b-versatile",
      "is_default": true
    },
    {
      "name": "gemini-flash",
      "provider": "google",
      "model": "gemini-2.0-flash",
      "is_default": false
    }
  ],
  "channels": {
    "telegram": {
      "enabled": true,
      "polling": true
    }
  },
  "redis": {
    "enabled": true,
    "prefix": "kios:"
  },
  "tools": {
    "read_file": true,
    "write_memory": true,
    "time": true
  }
}
```

### agent/Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache wget tar
RUN wget -q https://github.com/sipeed/picoclaw/releases/latest/download/picoclaw_Linux_amd64.tar.gz \
    && tar xzf picoclaw_Linux_amd64.tar.gz \
    && chmod +x picoclaw

FROM alpine:3.19
WORKDIR /kios
RUN apk add --no-cache ca-certificates tzdata bash
COPY --from=builder /app/picoclaw /usr/local/bin/picoclaw
COPY agent/ .
RUN chmod +x scripts/entrypoint.sh
ENV TZ=Asia/Makassar
EXPOSE 8080
CMD ["bash", "scripts/entrypoint.sh"]
```

### agent/scripts/entrypoint.sh

```bash
#!/bin/bash
set -e

# Validasi env wajib
: "${GROQ_API_KEY:?ERROR: GROQ_API_KEY belum diset}"
: "${TELEGRAM_BOT_TOKEN:?ERROR: TELEGRAM_BOT_TOKEN belum diset}"
: "${UPSTASH_REDIS_URL:?ERROR: UPSTASH_REDIS_URL belum diset}"
: "${UPSTASH_REDIS_TOKEN:?ERROR: UPSTASH_REDIS_TOKEN belum diset}"

echo "=== Mini Kios Agent Starting (WITA: $(TZ=Asia/Makassar date)) ==="

# Tulis config dari env ke file yang dibaca PicoClaw
cat > /kios/config/kios.json <<EOF
{
  "agent_file": "AGENT.md",
  "workspace": "workspace/",
  "model_list": [
    {
      "name": "groq-llama",
      "provider": "groq",
      "model": "llama-3.3-70b-versatile",
      "api_key": "${GROQ_API_KEY}",
      "is_default": true
    }
    $([ -n "${GEMINI_API_KEY}" ] && echo ',{"name":"gemini-flash","provider":"google","model":"gemini-2.0-flash","api_key":"'"${GEMINI_API_KEY}"'"}')
  ],
  "channels": {
    "telegram": {
      "enabled": true,
      "polling": true,
      "token": "${TELEGRAM_BOT_TOKEN}"
    }
  },
  "redis": {
    "enabled": true,
    "url": "${UPSTASH_REDIS_URL}",
    "token": "${UPSTASH_REDIS_TOKEN}",
    "prefix": "kios:"
  },
  "tools": {
    "read_file": true,
    "write_memory": true,
    "time": true
  }
}
EOF

echo "Config ditulis. Menjalankan picoclaw gateway..."
exec picoclaw gateway -E -config /kios/config/kios.json
```

### agent/.env.example

```env
# === WAJIB ===
GROQ_API_KEY=gsk_...              # dari console.groq.com (gratis)
TELEGRAM_BOT_TOKEN=...            # dari @BotFather di Telegram
UPSTASH_REDIS_URL=https://...     # dari console.upstash.com
UPSTASH_REDIS_TOKEN=...           # dari console.upstash.com

# === OPSIONAL ===
GEMINI_API_KEY=...                # fallback kalau Groq limit habis
KIOS_NAMA=Kios Bu Sari            # nama kios tampil di greeting
KIOS_ADMIN_IDS=123456789,987654321  # Telegram user ID yang jadi admin
```

---

## LANGKAH 3 — DASHBOARD KIOS (React + Vite)

Dashboard ini adalah UI utama kasir sehari-hari. Harus:
- **Cepat** — buka di browser lokal, tidak butuh internet untuk operasi dasar
- **Mobile-friendly** — bisa dipakai dari HP saat laptop sedang jalan sebagai server lokal
- **Terhubung ke Upstash Redis** — semua data real-time, tidak ada state lokal yang bisa hilang
- **Desain sudah ada** — ikuti desain kios-dashboard yang sudah disiapkan (lihat bagian DESAIN)

### dashboard/package.json

```json
{
  "name": "minikios-dashboard",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "dev": "vite --host",
    "build": "vite build",
    "preview": "vite preview --host"
  },
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "react-router-dom": "^7.0.0",
    "@upstash/redis": "^1.34.0",
    "date-fns": "^4.0.0",
    "lucide-react": "^0.469.0"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.3.0",
    "vite": "^6.0.0"
  }
}
```

### dashboard/vite.config.js

```js
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,        // bisa diakses dari HP di jaringan yang sama
    port: 5173,
  },
  define: {
    // env vars Upstash diakses via import.meta.env
  }
})
```

### dashboard/.env.example

```env
VITE_UPSTASH_REDIS_URL=https://...
VITE_UPSTASH_REDIS_TOKEN=...
VITE_KIOS_NAMA=Kios Bu Sari
VITE_KIOS_PASSWORD_ADMIN=admin123   # ganti setelah pertama kali
VITE_KIOS_PASSWORD_KASIR=kasir123   # ganti setelah pertama kali
```

### dashboard/src/lib/redis.js

Buat Redis client yang konek ke Upstash via REST API (bukan WebSocket, agar aman dari browser):

```js
// Upstash Redis REST client — berjalan langsung dari browser
// Tidak butuh backend Node.js

const REDIS_URL   = import.meta.env.VITE_UPSTASH_REDIS_URL
const REDIS_TOKEN = import.meta.env.VITE_UPSTASH_REDIS_TOKEN

async function redisCmd(...args) {
  const res = await fetch(`${REDIS_URL}/${args.map(encodeURIComponent).join('/')}`, {
    headers: { Authorization: `Bearer ${REDIS_TOKEN}` }
  })
  const json = await res.json()
  if (json.error) throw new Error(json.error)
  return json.result
}

// Key schema — semua pakai prefix "kios:"
// kios:transaksi:{YYYYMMDD}:{uuid}   → Hash detail transaksi
// kios:transaksi:idx:{YYYYMMDD}      → List uuid transaksi per hari
// kios:pelanggan:{id}                → Hash data pelanggan
// kios:pelanggan:idx                 → Set semua id pelanggan
// kios:utang:pembeli:{id}            → Hash utang per pelanggan
// kios:utang:supplier:{id}           → Hash utang ke supplier
// kios:stok:{produk_id}              → Hash data stok
// kios:supplier:{id}                 → Hash data supplier
// kios:user:{username}               → Hash data user (role, password hash)
// kios:harga                         → Hash semua harga produk
// kios:rekap:{YYYYMMDD}              → Hash rekap harian (total, count)

export const redis = {
  // Transaksi
  async simpanTransaksi(data) { /* HSET + LPUSH ke index */ },
  async getTransaksiHari(tanggal) { /* LRANGE + pipeline HGETALL */ },
  async getRekap(tanggal) { /* HGETALL kios:rekap:{tanggal} */ },

  // Pelanggan
  async simpanPelanggan(data) { /* HSET + SADD ke index */ },
  async getPelanggan(id) { /* HGETALL */ },
  async cariPelanggan(query) { /* scan + filter */ },
  async getAllPelanggan() { /* SMEMBERS + pipeline HGETALL */ },

  // Utang
  async simpanUtangPembeli(pelangganId, data) { /* HSET */ },
  async simpanUtangSupplier(supplierId, data) { /* HSET */ },
  async getUtangPembeli(pelangganId) { /* HGETALL */ },
  async getAllUtangPembeli() { /* scan kios:utang:pembeli:* */ },
  async getAllUtangSupplier() { /* scan kios:utang:supplier:* */ },
  async lunasUtang(type, id) { /* HSET status=lunas + timestamp */ },

  // Stok
  async getStok(produkId) { /* HGETALL kios:stok:{produkId} */ },
  async getAllStok() { /* scan kios:stok:* */ },
  async updateStok(produkId, jumlah, keterangan) { /* HSET + catat riwayat */ },

  // Supplier
  async simpanSupplier(data) { /* HSET */ },
  async getAllSupplier() { /* scan kios:supplier:* */ },

  // Harga
  async getHarga() { /* HGETALL kios:harga */ },
  async updateHarga(produk, harga) { /* HSET kios:harga */ },

  // User / Auth
  async getUser(username) { /* HGETALL kios:user:{username} */ },
  async updateUser(username, data) { /* HSET */ },

  // Backup
  async exportSemua() { /* scan semua key kios:* → JSON */ },
}
```

Implementasikan semua method di atas secara lengkap menggunakan Upstash REST API.
Gunakan pipeline (`/pipeline`) untuk batch request agar efisien.

### dashboard/src/lib/auth.js

```js
// Auth sederhana berbasis session storage + password hash
// Role: admin (akses penuh) | kasir (transaksi + utang saja)

export const auth = {
  login(username, password) { /* cek ke Redis, simpan session */ },
  logout() { /* hapus session */ },
  getUser() { /* baca dari sessionStorage */ },
  isAdmin() { /* cek role */ },
  requireAuth() { /* redirect ke login kalau belum login */ },
}
```

### dashboard/src/lib/format.js

```js
// Semua format yang dipakai di seluruh dashboard

export const formatRupiah = (nominal) =>
  new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0 }).format(nominal)

export const formatWITA = (date = new Date()) =>
  new Intl.DateTimeFormat('id-ID', {
    timeZone: 'Asia/Makassar',
    dateStyle: 'full',
    timeStyle: 'short'
  }).format(date)

export const tanggalKey = (date = new Date()) => {
  // Return YYYYMMDD dalam WITA — dipakai sebagai key Redis
  const d = new Date(date.toLocaleString('en-US', { timeZone: 'Asia/Makassar' }))
  return `${d.getFullYear()}${String(d.getMonth()+1).padStart(2,'0')}${String(d.getDate()).padStart(2,'0')}`
}

export const generateId = () => `${Date.now()}-${Math.random().toString(36).slice(2,7)}`
```

---

## LANGKAH 4 — HALAMAN DASHBOARD (Implementasi Penuh)

Implementasikan semua halaman berikut secara lengkap dan fungsional.

### dashboard/src/pages/Transaksi.jsx — HALAMAN UTAMA KASIR

Ini halaman yang paling sering dibuka. Desain harus CEPAT dan EFISIEN:

**Layout:**
- Kiri: form input transaksi cepat
- Kanan: daftar transaksi hari ini (real-time dari Redis)

**Form transaksi:**
```
Jenis:     [Pulsa] [Paket Data] [Token PLN] [Transfer]
Pelanggan: [input nama / cari] + tombol [+ Baru]
Nomor HP:  [input] (auto-isi kalau pelanggan dipilih)
Nominal:   [dropdown dari harga.md] atau [input manual]
           → otomatis tampil: Harga Rp X + Admin Rp Y = Total Rp Z
Bayar:     [input] → Kembalian: Rp ...
Catatan:   [input opsional]
Status:    [Lunas] [Belum Bayar / Utang]

[CATAT TRANSAKSI] ← tombol besar, hijau
```

**Daftar transaksi hari ini:**
- Tabel: Waktu | Pelanggan | Produk | Nominal | Total | Kasir | Status
- Total omzet hari ini di bagian atas (update real-time)
- Filter: semua / lunas / utang
- Klik baris → detail + tombol edit status

**Fitur tambahan:**
- Tombol "Rekap Hari Ini" → modal summary
- Auto-refresh setiap 30 detik
- Shortcut keyboard: Enter untuk catat, Esc untuk batal

### dashboard/src/pages/Pelanggan.jsx

- Tabel semua pelanggan: Nama | No HP | Total Transaksi | Utang | Terakhir Beli
- Search realtime
- Klik pelanggan → detail: riwayat transaksi + utang aktif
- Form tambah/edit pelanggan
- Tombol: Lihat Utang, Tambah Transaksi

### dashboard/src/pages/Utang.jsx

Dua tab: **Utang Pembeli** | **Utang ke Supplier**

**Utang Pembeli:**
- Kartu per pelanggan: Nama | Total Utang | Sejak | Jatuh Tempo
- Warna merah kalau lewat jatuh tempo
- Tombol: Bayar Sebagian | Lunas | Lihat Rincian
- Filter: semua / aktif / lunas

**Utang ke Supplier:**
- Kartu per supplier: Nama | Total Utang | Tanggal
- Tombol: Catat Bayar | Lunas

### dashboard/src/pages/Stok.jsx

- Tabel stok: Produk | Stok | Minimum | Status | Terakhir Update
- Baris merah kalau stok ≤ minimum
- Tombol: Update Stok (masuk / keluar / koreksi)
- Riwayat perubahan stok per produk
- Alert kalau ada stok kritis

### dashboard/src/pages/Supplier.jsx

- Kartu per supplier: Nama | No HP | Produk Disuplai | Total Utang
- Form tambah/edit supplier
- Riwayat pembelian dari supplier
- Tombol: Catat Pembelian, Catat Bayar Utang

### dashboard/src/pages/Rekap.jsx

- Pilih tanggal / rentang tanggal
- Ringkasan: Total Omzet | Total Transaksi | Keuntungan Estimasi
- Breakdown per jenis produk (pulsa / paket / token / transfer)
- Tabel transaksi lengkap
- Tombol: Export ke CSV, Print

### dashboard/src/pages/Users.jsx — ADMIN ONLY

- Tabel user: Username | Role | Terakhir Login | Status
- Form tambah user (admin atau kasir)
- Reset password
- Aktif/nonaktifkan akun
- Log aktivitas: siapa catat apa kapan

### dashboard/src/pages/Backup.jsx — ADMIN ONLY

- Tombol: Export Semua Data (download JSON)
- Tombol: Export Transaksi Bulan Ini (CSV)
- Status backup terakhir
- Import data dari file JSON (restore)
- Tombol: Test koneksi Redis

---

## LANGKAH 5 — KOMPONEN SHARED

### dashboard/src/components/Sidebar.jsx

Sidebar navigasi dengan:
- Logo / nama kios di atas
- Menu: Transaksi (default) | Pelanggan | Utang | Stok | Supplier | Rekap | [Admin: Users, Backup]
- Badge merah di menu Utang kalau ada utang jatuh tempo
- Badge kuning di menu Stok kalau ada stok kritis
- Info user login di bawah (nama, role, tombol logout)
- Collapsible di mobile

### dashboard/src/components/Header.jsx

- Tanggal & jam WITA (update tiap detik)
- Nama kios
- Status koneksi Redis (hijau/merah)
- Nama user & role

---

## LANGKAH 6 — DESAIN DASHBOARD

**Ikuti desain kios-dashboard yang sudah ada.** Implementasikan dengan panduan ini:

**Palet warna:**
```css
:root {
  --hijau-kios: #16a34a;       /* aksi utama, tombol catat */
  --hijau-muda: #dcfce7;       /* background sukses */
  --biru-info:  #2563eb;       /* info, link */
  --kuning-warn: #d97706;      /* stok menipis, perhatian */
  --merah-danger: #dc2626;     /* utang jatuh tempo, error */
  --abu-bg:     #f8fafc;       /* background halaman */
  --abu-card:   #ffffff;       /* background card */
  --abu-border: #e2e8f0;       /* border */
  --teks-utama: #0f172a;       /* teks primer */
  --teks-muted: #64748b;       /* teks sekunder */
}
```

**Typography:**
- Font: `'Plus Jakarta Sans', sans-serif` (import dari Google Fonts)
- Heading halaman: 20px / 600
- Label form: 14px / 500
- Data tabel: 14px / 400
- Angka rupiah: monospace, 16px / 600

**Komponen UI:**
```css
/* Card */
.card {
  background: var(--abu-card);
  border: 1px solid var(--abu-border);
  border-radius: 12px;
  padding: 16px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.04);
}

/* Tombol utama (Catat Transaksi) */
.btn-primary {
  background: var(--hijau-kios);
  color: white;
  border-radius: 8px;
  padding: 12px 24px;
  font-size: 16px;
  font-weight: 600;
  width: 100%;
  cursor: pointer;
  transition: background 0.15s;
}
.btn-primary:hover { background: #15803d; }

/* Badge status */
.badge-lunas   { background: #dcfce7; color: #16a34a; }
.badge-utang   { background: #fef2f2; color: #dc2626; }
.badge-pending { background: #fefce8; color: #d97706; }
```

**Layout responsif:**
```css
/* Desktop: sidebar kiri 240px + konten kanan */
/* Tablet: sidebar collapsible */
/* Mobile: bottom navigation, form full-width */
@media (max-width: 768px) {
  .sidebar { /* bottom nav */ }
  .transaksi-layout { /* stack vertikal */ }
}
```

**Micro-interactions:**
- Animasi toast saat transaksi berhasil dicatat (hijau, 3 detik)
- Loading skeleton saat fetch Redis
- Konfirmasi dialog sebelum hapus/lunas
- Form reset setelah transaksi berhasil

---

## LANGKAH 7 — LOGIN PAGE

### dashboard/src/pages/Login.jsx

```
┌─────────────────────────────┐
│  🏪 Mini Kios Desa          │
│  [Nama Kios]                │
│                             │
│  Username: [___________]    │
│  Password: [___________]    │
│                             │
│  [MASUK]                    │
│                             │
│  v1.0.0 · Rote Ndao, NTT   │
└─────────────────────────────┘
```

- Login cek username/password ke Redis (`kios:user:{username}`)
- Simpan session ke sessionStorage (bukan localStorage)
- Redirect ke /transaksi setelah login
- Pesan error: "Username atau password salah"
- Tidak ada "lupa password" — reset via admin di halaman Users

---

## LANGKAH 8 — ROUTING & APP

### dashboard/src/App.jsx

```jsx
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'

// Protected route: redirect ke /login kalau belum auth
// Admin route: redirect ke /transaksi kalau bukan admin

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route element={<ProtectedLayout />}>
          <Route path="/" element={<Navigate to="/transaksi" />} />
          <Route path="/transaksi" element={<Transaksi />} />
          <Route path="/pelanggan" element={<Pelanggan />} />
          <Route path="/utang" element={<Utang />} />
          <Route path="/stok" element={<Stok />} />
          <Route path="/supplier" element={<Supplier />} />
          <Route path="/rekap" element={<Rekap />} />
          {/* Admin only */}
          <Route element={<AdminRoute />}>
            <Route path="/users" element={<Users />} />
            <Route path="/backup" element={<Backup />} />
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
```

---

## LANGKAH 9 — SETUP AWAL DATA REDIS

Buat script `dashboard/src/lib/setup.js` yang dijalankan sekali untuk:

1. Buat user admin default:
   ```
   kios:user:admin → { role: "admin", password: hash("admin123"), nama: "Admin" }
   kios:user:kasir → { role: "kasir", password: hash("kasir123"), nama: "Kasir" }
   ```
   ⚠️ Tampilkan peringatan besar di UI untuk ganti password setelah pertama login

2. Import harga dari `workspace/harga.md` → `kios:harga` di Redis

3. Setup stok awal produk fisik (kartu perdana, dll)

Buat juga halaman `/setup` yang hanya bisa diakses kalau belum ada user di Redis.

---

## LANGKAH 10 — CI/CD & DEPLOY

### .github/workflows/deploy.yml

```yaml
name: Deploy ke Railway

on:
  push:
    branches: [main]
    paths:
      - 'agent/**'
      - 'Dockerfile'

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Deploy ke Railway
        uses: bervProject/railway-deploy@main
        with:
          railway_token: ${{ secrets.RAILWAY_TOKEN }}
          service: minikios-agent
```

### docs/DEPLOY_RAILWAY.md

Tulis panduan deploy Railway langkah-demi-langkah:

1. Buat akun Railway → New Project → Deploy from GitHub
2. Set environment variables:
   - `GROQ_API_KEY`
   - `TELEGRAM_BOT_TOKEN`
   - `UPSTASH_REDIS_URL`
   - `UPSTASH_REDIS_TOKEN`
   - `GEMINI_API_KEY` (opsional)
3. Railway otomatis build Dockerfile dari folder `agent/`
4. Verifikasi: kirim `/start` ke bot Telegram
5. Cara update: `git push origin main` → Railway auto-deploy

### docs/CARA_PAKAI.md

Panduan kasir sehari-hari (bahasa sederhana):

1. **Buka dashboard** — ketik `localhost:5173` di browser
2. **Login** — masukkan username dan password
3. **Catat transaksi** — isi form di halaman Transaksi, klik Catat
4. **Catat utang** — saat catat transaksi, pilih status "Belum Bayar"
5. **Tandai lunas** — buka halaman Utang, klik Lunas
6. **Cek stok** — halaman Stok, perhatikan yang merah
7. **Rekap harian** — halaman Rekap, pilih tanggal hari ini
8. **Via Telegram** — pelanggan bisa tanya harga langsung ke bot

---

## LANGKAH 11 — README & DOKUMENTASI

### README.md (root project)

Tulis README lengkap mencakup:
- Deskripsi singkat sistem
- Arsitektur diagram ASCII
- Cara setup cepat (5 langkah)
- Environment variables yang dibutuhkan
- Cara jalankan dashboard lokal
- Cara deploy ke Railway
- Cara update harga
- Link ke docs/ untuk panduan lengkap

### ROADMAP.md

Update roadmap dari versi lama, tambahkan:
- Fase 0 selesai: arsitektur Railway + Redis + Dashboard
- Fase 1: WhatsApp channel, struk digital via Telegram, reminder utang otomatis
- Fase 2: laporan bulanan PDF, multi-kios, integrasi supplier digital

---

## LANGKAH 12 — .gitignore

```gitignore
# Environment
.env
.env.local
.env.*.local

# Dependencies
node_modules/
dashboard/node_modules/

# Build output
dashboard/dist/
dashboard/.vite/

# PicoClaw runtime
*.jsonl
picoclaw
picoclaw-launcher
build/

# OS
.DS_Store
Thumbs.db

# IDE
.vscode/
.idea/
*.swp

# Logs
*.log
```

---

## URUTAN EKSEKUSI

Jalankan dalam urutan ini:

```bash
# 1. Setup project
cd ~/Publik/project/minikios
git init

# 2. Buat semua file dan direktori sesuai struktur di atas

# 3. Setup dashboard
cd dashboard
npm install
cp .env.example .env
# Edit .env → isi VITE_UPSTASH_REDIS_URL dan VITE_UPSTASH_REDIS_TOKEN

# 4. Jalankan dashboard lokal
npm run dev
# Buka http://localhost:5173

# 5. Setup Redis awal (buka /setup di browser pertama kali)

# 6. Commit dan push ke GitHub
cd ..
git add .
git commit -m "feat: initial mini kios desa setup"
git push origin main

# 7. Deploy ke Railway
# Ikuti docs/DEPLOY_RAILWAY.md

# 8. Test: kirim pesan ke Telegram bot
```

---

## CATATAN PENTING

1. **Satu bot token, satu poller** — Railway yang jalan sebagai long poller Telegram.
   Jangan jalankan picoclaw gateway di lokal dengan token yang sama.

2. **Dashboard lokal tidak butuh internet** untuk tampil, tapi butuh koneksi ke Upstash Redis
   untuk baca/tulis data. Kalau internet mati, tampilkan pesan offline yang jelas.

3. **Upstash Redis free tier** — 10.000 request/hari, 256MB storage. Untuk kios kecil
   di desa dengan volume rendah, ini lebih dari cukup.

4. **Keamanan** — password di Redis disimpan sebagai bcrypt hash. Jangan simpan password
   plaintext. Jangan commit file .env ke GitHub.

5. **Timezone** — SEMUA waktu disimpan dan ditampilkan dalam WITA (Asia/Makassar, UTC+8).
   Gunakan `formatWITA()` dari `lib/format.js` di seluruh aplikasi.

6. **PicoClaw Web UI** (port 18800) adalah terpisah dari Dashboard Kios (port 5173).
   Keduanya bisa jalan bersamaan. Web UI hanya untuk konfigurasi agent di Railway.

7. **Backup** — ekspor manual dari halaman Backup di dashboard. Data aman di Upstash Redis
   (mereka punya replikasi sendiri), tapi export rutin ke JSON tetap disarankan.
