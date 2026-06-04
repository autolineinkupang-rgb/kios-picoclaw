# Panduan Perintah Kios 🏪

Panduan ini untuk **pemilik kios dan kasir** — tidak perlu paham teknis.
Semua dilakukan lewat chat **Telegram** ke bot kios.

Ada dua cara pakai bot:

1. **Ngobrol biasa** — tulis seperti bicara ke orang. Contoh: *"jual beras 2 karung"*,
   *"stok gula tinggal berapa?"*, *"tambah produk minyak goreng harga 18rb stok 24"*.
   Bot pakai AI untuk mengerti maksud kamu.
2. **Perintah cepat (garis miring `/`)** — lebih cepat dan **tetap jalan walau AI sedang sibuk/limit**.
   Tinggal ketik perintahnya. Daftarnya di bawah.

---

## 🟢 Perintah cepat untuk semua (kasir & pemilik)

| Perintah | Gunanya | Contoh |
|----------|---------|--------|
| `/stok` | Lihat semua stok | `/stok` |
| `/stok <nama>` | Cari stok satu barang | `/stok beras` |
| `/produk` | Lihat / cari daftar produk | `/produk gula` |
| `/menipis` | Lihat barang yang stoknya hampir habis | `/menipis` |
| `/harga <nama>` | Cek harga jual barang | `/harga minyak` |
| `/jual <barang> <jumlah>` | Jual cepat + struk | `/jual beras 2` |
| `/jualmassal <barang> <jml>, ...` | Jual beberapa barang sekaligus | `/jualmassal beras 2, gula 3` |
| `/shift` | Lihat status shift kasir | `/shift` |
| `/laporan` | Ringkasan penjualan & laba hari ini | `/laporan` |
| `/promo` | Lihat promo yang sedang aktif | `/promo` |
| `/pasar` | Bandingkan harga kita vs harga pasar | `/pasar` |
| `/suplier` | Lihat / cari supplier | `/suplier` |
| `/suplier banding <barang>` | Bandingkan harga antar supplier | `/suplier banding gula` |
| `/template` | Minta file Excel untuk isi data massal | `/template` |

> 💡 Mau isi banyak produk/supplier sekaligus? Ketik `/template`, isi file Excel-nya,
> lalu kirim balik filenya ke chat — bot akan import otomatis.

---

## 🔑 Perintah khusus pemilik (owner)

Hanya bisa dilakukan oleh **pemilik**. Kasir tidak bisa.

### Kelola data (ngobrol biasa ke bot)
- **Tambah/ubah/hapus produk**, ubah harga, atur promo.
- **Tambah/nonaktifkan kasir** dan atur perannya (kasir / owner).
  Contoh: *"tambah kasir Budi, ID telegram 123456789"*.
- **Batalkan transaksi** yang salah.

### Backup & Restore data 💾 (PENTING)
Data kios disimpan di server (Upstash Redis). **Kalau kredit server habis,
data bisa terhapus setelah 30 hari.** Karena itu rajinlah backup.

| Aksi | Cara |
|------|------|
| **Backup sekarang** | Ketik `/backup`. Bot kirim file `.json` berisi semua data ke chat. **Simpan file itu baik-baik** (jangan dihapus dari Telegram). |
| **Backup otomatis harian** | Otomatis kalau sudah diatur saat deploy (lihat bagian Pengaturan). Bot kirim file backup tiap hari. |
| **Pulihkan (restore)** | Kirim file backup `.json` ke chat, lalu bilang *"tolong pulihkan data dari file ini"*. Bot tampilkan ringkasan dulu. Kalau yakin, konfirmasi dengan *"ya, restore paksa"*. |

> ⚠️ **Restore menimpa SEMUA data** yang ada sekarang dengan isi file — tidak bisa
> dibatalkan. Pakai hanya kalau data hilang atau rusak.

---

## ⚙️ Pengaturan keamanan (untuk yang setup deploy)

Diatur lewat **Environment Variables** di Railway. Disarankan untuk keamanan harian:

| Variabel | Gunanya | Contoh |
|----------|---------|--------|
| `KIOS_OWNER_IDS` | Daftar ID Telegram pemilik (dipisah koma). ID ini **selalu owner** dan tak bisa terkunci. | `123456789,987654321` |
| `KIOS_DEFAULT_ROLE` | Peran default untuk orang lain di whitelist. Set `kasir` supaya hanya owner yang bisa hapus/batalkan. | `kasir` |
| `KIOS_BACKUP_CHAT` | ID chat tujuan backup otomatis harian. Kosongkan untuk matikan. | `123456789` |
| `KIOS_BACKUP_CRON` | Jam backup otomatis (format cron). Default `0 22 * * *` (jam 22:00 WITA). | `0 22 * * *` |
| `KIOS_REPORT_CHAT` | ID chat tujuan laporan harian otomatis. | `123456789` |

> 🔒 **Cara aman mengunci kios:** isi `KIOS_OWNER_IDS` dengan ID Telegram kamu **dulu**,
> baru set `KIOS_DEFAULT_ROLE=kasir`. Dengan begitu kamu tetap owner penuh, sementara
> orang lain hanya bisa transaksi biasa (tidak bisa hapus data).

> Cara tahu ID Telegram: chat ke bot, atau pakai bot `@userinfobot`.
