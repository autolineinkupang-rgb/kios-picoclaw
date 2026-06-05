import type { Metadata } from "next";
import { getAllPelanggan, getAllPiutang } from "@/lib/kios";
import { ConnectionError } from "@/components/connection-error";
import { PelangganList } from "@/components/pelanggan/pelanggan-list";

export const metadata: Metadata = { title: "Pelanggan" };
export const dynamic = "force-dynamic";

export default async function PelangganPage() {
  let pelanggan, piutang;
  try {
    [pelanggan, piutang] = await Promise.all([getAllPelanggan(), getAllPiutang()]);
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  const piutangTerbuka = piutang.filter((p) => p.status === "terbuka" && p.sisa > 0);
  const totalPiutang = piutangTerbuka.reduce((sum, p) => sum + p.sisa, 0);
  const pelangganBerhutang = new Set(piutangTerbuka.map((p) => p.pelanggan_id)).size;

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Pelanggan</h2>
        <p className="text-sm text-muted-foreground">
          Daftar pembeli yang pernah memesan. Klik baris untuk melihat riwayat pesanan.
        </p>
      </div>
      <PelangganList pelanggan={pelanggan} totalPiutang={totalPiutang} pelangganBerhutang={pelangganBerhutang} />
    </div>
  );
}
