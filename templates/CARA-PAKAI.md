# Cara Pakai Template (Isi Data Massal ke Kios)

Panduan mengisi template dan memasukkan banyak produk/supplier sekaligus ke bot kios.
File template ada di folder ini (`templates/`):

| File | Untuk | Format |
|---|---|---|
| `produk-template.xlsx` / `.csv` | daftar produk | Excel / CSV |
| `supplier-template.xlsx` / `.csv` | daftar supplier | Excel / CSV |

Pakai yang `.xlsx` kalau mau diisi di Excel/Google Sheets (lebih rapi). Yang `.csv` kalau suka teks polos.

---

## 1. Isi template

### Produk — arti tiap kolom
| Kolom | Wajib? | Contoh | Keterangan |
|---|---|---|---|
| `nama` | ✅ wajib | Beras Medium 5kg | Nama produk. **Kunci pencocokan** — nama sama = produk yang sama. |
| `barcode` | opsional | 8991234500015 | Kode barcode (buat scan/cari cepat). Boleh dikosongkan. |
| `kategori` | opsional | sembako | Bebas (sembako, mie, minuman, dll). Kosong → "umum". |
| `satuan` | opsional | karung | pcs/botol/karung/bungkus. Kosong → "pcs". |
| `stok` | opsional | 20 | Jumlah stok saat ini (angka). |
| `harga_beli` | opsional | 55000 | Harga modal per satuan (angka, tanpa "Rp"/titik). |
| `harga_jual` | opsional | 62000 | Harga jual per satuan. |
| `stok_minimum` | opsional | 10 | Batas stok mulai diingatkan. Kosong → 5. |
| `stok_kritis` | opsional | 3 | Batas stok kritis/hampir habis. Kosong → 2. |
| `supplier` | opsional | UD Maju | Nama pemasok. |
| `gambar` | opsional | https://contoh.com/beras.jpg | URL gambar produk untuk tampil di toko pembeli. Boleh dikosongkan. |

### Supplier — arti tiap kolom
| Kolom | Wajib? | Contoh |
|---|---|---|
| `nama` | ✅ wajib | UD Maju |
| `kontak` | opsional | 0812xxxxxxx |
| `alamat` | opsional | Baa Rote Ndao |
| `produk_utama` | opsional | beras gula minyak |
| `catatan` | opsional | MOQ 10 karung; lead time 2 hari |

**Aturan penting:**
- Baris contoh di template **boleh dihapus**, ganti dengan data kamu.
- Angka **tanpa "Rp" dan tanpa titik ribuan** (tulis `15000`, bukan `Rp 15.000`).
- **Jangan ubah nama kolom** (baris pertama/header) — biarkan apa adanya.
- Baris tanpa `nama` akan **dilewati**.
- Kalau pakai Excel `.xlsx`, simpan biasa. Kalau diminta format, pilih `.xlsx` atau `.csv`.

---

## 2. Siapkan URL Redis (sekali saja)

Importer butuh `UPSTASH_REDIS_URL`. Simpan di file lokal `.env` (aman, tidak ke-commit):
```bash
printf 'export UPSTASH_REDIS_URL="rediss://default:PASSWORD@xxx.upstash.io:6379"\n' > ~/kios-picoclaw/.env
```
> URL Redis ambil dari https://console.upstash.com (bentuk `rediss://...`).

---

## 3. Import ke Redis

Jalankan di komputermu (bukan di Railway):
```bash
cd ~/kios-picoclaw
source .env

# produk (boleh .xlsx atau .csv)
~/sdk/go/bin/go run ./cmd/kios-import produk daftar-produk.xlsx

# supplier
~/sdk/go/bin/go run ./cmd/kios-import supplier daftar-supplier.xlsx
```

Ganti `daftar-produk.xlsx` dengan nama file kamu.
Hasil di layar: `Selesai ✅  Dibuat: X | Diupdate: Y | Dilewati: Z`.

**Cara kerja import (aman):**
- Dicocokkan per **nama** (huruf besar/kecil diabaikan): nama yang sudah ada → **di-update**, nama baru → **dibuat** (id otomatis).
- **Kolom kosong tidak menimpa** nilai lama → bisa update sebagian (mis. cuma ubah harga, kolom lain dikosongkan).
- Bisa di-import **berulang** tanpa bikin duplikat.

---

## 4. Cek hasilnya

Di Telegram, kirim ke bot:
- `/produk` → daftar semua produk
- `/suplier` → daftar semua supplier
- atau tanya biasa: "ada stok apa aja?"

---

## Masalah umum
| Gejala | Sebab & solusi |
|---|---|
| `Dilewati: N` banyak | Ada baris tanpa `nama`. Isi kolom nama. |
| Harga jadi aneh | Angka pakai "Rp"/titik. Tulis polos: `15000`. |
| `tidak bisa konek Redis` | `UPSTASH_REDIS_URL` salah / belum `source .env`. Pastikan `rediss://`. |
| Header error / kolom tak terbaca | Nama kolom (baris pertama) diubah. Pakai header asli template. |

## Regenerasi template Excel
Kalau mau ubah kolom/contoh:
```bash
~/sdk/go/bin/go run ./cmd/gen-templates
```
