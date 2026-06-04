import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getAllPesanan, getPelanggan } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { PelangganDetail } from "@/components/pelanggan/pelanggan-detail";

export const metadata: Metadata = { title: "Detail Pelanggan" };
export const dynamic = "force-dynamic";

export default async function PelangganDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const phone = decodeURIComponent(id);

  let pelanggan;
  let pesanan;
  try {
    [pelanggan, pesanan] = await Promise.all([
      getPelanggan(phone),
      getAllPesanan(),
    ]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  if (!pelanggan) notFound();

  const riwayat = pesanan.filter((p) => p.pelanggan_id === pelanggan.id);

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">{pelanggan.nama}</h2>
        <p className="text-sm text-muted-foreground font-mono">{pelanggan.phone}</p>
      </div>
      <PelangganDetail pelanggan={pelanggan} riwayat={riwayat} />
    </div>
  );
}
