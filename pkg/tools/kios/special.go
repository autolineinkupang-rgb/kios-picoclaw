package kios

import (
	"context"
	"fmt"
	"math"
)

// sellPulsa records a sale of one pulsa nominal.
//
// Flow:
//  1. Lookup PulsaDenom(nominal) — error if absent or inactive.
//  2. Validate item.SaldoModal >= denom.HargaModal.
//  3. Use DecrSaldoModal (atomic read-modify-write under store.mu).
//  4. Reload item to get fresh SaldoModal.
//  5. Append transaction.
//  6. Return (tx, item_updated, sisa_saldo, nil).
func sellPulsa(ctx context.Context, store *Store, item *Produk, nominal int, metode, kasir string) (*Transaksi, *Produk, int, error) {
	denom, err := store.GetPulsaDenom(ctx, nominal)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("gagal baca denom pulsa: %w", err)
	}
	if denom == nil {
		return nil, nil, 0, fmt.Errorf("nominal pulsa %s tidak tersedia kak 🙏 cek /pulsa untuk daftar nominal", FormatRupiah(nominal))
	}
	if !denom.Aktif {
		return nil, nil, 0, fmt.Errorf("nominal pulsa %s sedang tidak aktif kak 😔", FormatRupiah(nominal))
	}
	if item.SaldoModal < denom.HargaModal {
		return nil, nil, 0, fmt.Errorf("modal pulsa kurang kak 😔 saldo modal %s, butuh %s — minta owner isi dulu ya",
			FormatRupiah(item.SaldoModal), FormatRupiah(denom.HargaModal))
	}

	if err := store.DecrSaldoModal(ctx, item.ID, denom.HargaModal); err != nil {
		return nil, nil, 0, fmt.Errorf("gagal kurangi saldo modal: %w", err)
	}
	// Reload to get the fresh SaldoModal after atomic decr.
	updated, err := store.GetProduk(ctx, item.ID)
	if err != nil || updated == nil {
		return nil, nil, 0, fmt.Errorf("gagal reload produk setelah kurangi saldo: %w", err)
	}

	now := NowWITA()
	tx := &Transaksi{
		Tanggal:     now.Format("2006-01-02"),
		Jam:         now.Format("15:04:05"),
		ProdukID:    item.ID,
		NamaProduk:  item.Nama,
		Kategori:    "pulsa",
		Qty:         1,
		HargaSatuan: denom.HargaJual,
		Total:       denom.HargaJual,
		Modal:       denom.HargaModal,
		MetodeBayar: metode,
		Kasir:       kasir,
		Catatan:     fmt.Sprintf("nominal %s", FormatRupiah(nominal)),
	}
	if _, err := store.AppendTransaksi(ctx, tx); err != nil {
		// Rollback saldo (best-effort; very rare)
		_ = store.IncrSaldoModal(ctx, item.ID, denom.HargaModal)
		return nil, nil, 0, fmt.Errorf("gagal catat transaksi pulsa: %w", err)
	}
	_ = store.TrackHabit(ctx, "sale", item.Nama)
	return tx, updated, updated.SaldoModal, nil
}

// sellBensin records a sale of bensin in mili-liters.
//
// Flow:
//  1. Validate item.StokMl >= ml.
//  2. Decrement StokMl, sync Stok = StokMl/1000, SetProduk.
//  3. Compute total and modal using math.Round.
//  4. Append transaction with Liter field.
//  5. Return (tx, item_updated, sisa_ml, nil).
func sellBensin(ctx context.Context, store *Store, item *Produk, ml int, metode, kasir string) (*Transaksi, *Produk, int, error) {
	if ml <= 0 {
		return nil, nil, 0, fmt.Errorf("volume bensin harus lebih dari 0 kak 🙏")
	}
	if item.StokMl < ml {
		liter := float64(item.StokMl) / 1000
		return nil, nil, 0, fmt.Errorf("stok bensin %s tidak cukup kak 😔 sisa %.2fL", item.Nama, liter)
	}

	now := NowWITA()
	item.StokMl -= ml
	item.Stok = item.StokMl / 1000
	item.LastUpdate = now.Format("2006-01-02")
	if err := store.SetProduk(ctx, item); err != nil {
		return nil, nil, 0, fmt.Errorf("gagal update stok bensin: %w", err)
	}

	liter := float64(ml) / 1000
	total := int(math.Round(float64(item.HargaJual) * liter))
	modal := int(math.Round(float64(item.HargaBeli) * liter))

	tx := &Transaksi{
		Tanggal:     now.Format("2006-01-02"),
		Jam:         now.Format("15:04:05"),
		ProdukID:    item.ID,
		NamaProduk:  item.Nama,
		Kategori:    "bensin",
		Qty:         1,
		HargaSatuan: item.HargaJual,
		Total:       total,
		Modal:       modal,
		Liter:       liter,
		MetodeBayar: metode,
		Kasir:       kasir,
		Catatan:     fmt.Sprintf("%.3fL", liter),
	}
	if _, err := store.AppendTransaksi(ctx, tx); err != nil {
		// Rollback stok (best-effort)
		item.StokMl += ml
		item.Stok = item.StokMl / 1000
		_ = store.SetProduk(ctx, item)
		return nil, nil, 0, fmt.Errorf("gagal catat transaksi bensin: %w", err)
	}
	_ = store.TrackHabit(ctx, "sale", item.Nama)
	return tx, item, item.StokMl, nil
}
