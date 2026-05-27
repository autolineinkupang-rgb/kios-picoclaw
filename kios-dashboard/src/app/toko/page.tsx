import type { Metadata } from "next";
import { Store } from "lucide-react";
import { getAllProduk } from "@/lib/kios";
import { TokoView, type PublicProduk } from "@/components/toko/toko-view";

export const metadata: Metadata = {
  title: "Toko Kios Cerdas",
  description: "Pesan produk kios dengan mudah.",
};
export const dynamic = "force-dynamic";

export default async function TokoPage() {
  let items: PublicProduk[] = [];
  let gagal = false;
  try {
    const produk = await getAllProduk();
    // Public-safe projection — never expose cost price / supplier.
    items = produk.map((p) => ({
      id: p.id,
      nama: p.nama,
      kategori: p.kategori,
      satuan: p.satuan,
      harga_jual: p.harga_jual,
      stok: p.stok,
    }));
  } catch {
    gagal = true;
  }

  return (
    <div className="min-h-dvh bg-background">
      <header className="sticky top-0 z-30 border-b bg-background/95 backdrop-blur">
        <div className="mx-auto flex h-16 max-w-5xl items-center gap-3 px-4">
          <div className="flex size-9 items-center justify-center rounded-lg bg-accent text-accent-foreground">
            <Store className="size-5" aria-hidden />
          </div>
          <div className="leading-tight">
            <p className="text-sm font-semibold">Kios Cerdas</p>
            <p className="text-xs text-muted-foreground">Rote Ndao · Pesan online</p>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-4 py-6">
        {gagal ? (
          <div className="rounded-xl border bg-card p-8 text-center">
            <p className="font-medium">Toko belum siap</p>
            <p className="mt-1 text-sm text-muted-foreground">
              Maaf, daftar produk belum bisa dimuat. Coba lagi sebentar ya.
            </p>
          </div>
        ) : (
          <TokoView produk={items} />
        )}
      </main>
    </div>
  );
}
