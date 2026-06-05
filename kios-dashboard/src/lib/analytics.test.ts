import test from "node:test";
import assert from "node:assert/strict";
import { jenisFromCatatan, modalPerJenis } from "./analytics.ts";
import type { Transaksi, Produk } from "./types.ts";

test("jenisFromCatatan: returns jenis from bracket", () => {
  assert.equal(jenisFromCatatan("via dashboard [pulsa]"), "pulsa");
  assert.equal(jenisFromCatatan("via dashboard [bensin]"), "bensin");
  assert.equal(jenisFromCatatan("via dashboard [solar]"), "solar");
  assert.equal(jenisFromCatatan("via dashboard [minyak_tanah]"), "minyak_tanah");
});

test("jenisFromCatatan: returns biasa when no bracket", () => {
  assert.equal(jenisFromCatatan("via dashboard"), "biasa");
  assert.equal(jenisFromCatatan("via bot"), "biasa");
  assert.equal(jenisFromCatatan(""), "biasa");
});

test("modalPerJenis: splits modal by jenis from catatan", () => {
  const produk: Produk[] = [
    { id: "P1", nama: "Beras", kategori: "sembako", satuan: "kg", stok: 10, harga_beli: 10000, harga_jual: 12000, stok_minimum: 5, stok_kritis: 2, supplier: "", last_update: "", has_exp: false, exp_date: "", image_url: "", barcode: "" },
    { id: "P2", nama: "Pulsa", kategori: "pulsa", satuan: "paket", stok: 10, harga_beli: 25000, harga_jual: 27000, stok_minimum: 5, stok_kritis: 2, supplier: "", last_update: "", has_exp: false, exp_date: "", image_url: "", barcode: "", jenis: "pulsa" },
  ];
  const txs: Transaksi[] = [
    { id: "T1", tanggal: "2026-06-05", jam: "10:00:00", produk_id: "P1", nama_produk: "Beras", kategori: "sembako", qty: 2, harga_satuan: 12000, total: 24000, metode_bayar: "tunai", kasir: "admin", catatan: "via dashboard", session_id: "", modal: 20000 },
    { id: "T2", tanggal: "2026-06-05", jam: "10:05:00", produk_id: "P2", nama_produk: "Pulsa", kategori: "pulsa", qty: 1, harga_satuan: 27000, total: 27000, metode_bayar: "transfer", kasir: "admin", catatan: "via dashboard [pulsa]", session_id: "", modal: 25000 },
  ];
  const result = modalPerJenis(txs, produk);
  assert.equal(result.biasa, 20000);
  assert.equal(result.pulsa, 25000);
  assert.equal(result.bensin, 0);
  assert.equal(result.solar, 0);
  assert.equal(result.minyak_tanah, 0);
  assert.equal(result.total, 45000);
});

test("modalPerJenis: falls back to harga_beli when tx.modal not set", () => {
  const produk: Produk[] = [
    { id: "P1", nama: "Beras", kategori: "sembako", satuan: "kg", stok: 10, harga_beli: 10000, harga_jual: 12000, stok_minimum: 5, stok_kritis: 2, supplier: "", last_update: "", has_exp: false, exp_date: "", image_url: "", barcode: "" },
  ];
  const txs: Transaksi[] = [
    { id: "T1", tanggal: "2026-06-05", jam: "10:00:00", produk_id: "P1", nama_produk: "Beras", kategori: "sembako", qty: 3, harga_satuan: 12000, total: 36000, metode_bayar: "tunai", kasir: "admin", catatan: "via bot", session_id: "" },
  ];
  const result = modalPerJenis(txs, produk);
  assert.equal(result.biasa, 30000); // 3 * 10000 fallback
  assert.equal(result.total, 30000);
});
