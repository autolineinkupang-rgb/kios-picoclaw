import type { Metadata } from "next";
import { getConfig } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { AccessDenied } from "@/components/access-denied";
import { PengaturanForm } from "@/components/pengaturan/pengaturan-form";

export const metadata: Metadata = { title: "Pengaturan" };
export const dynamic = "force-dynamic";

export default async function PengaturanPage() {
  const session = await getSession();
  if (session?.role !== "owner") return <AccessDenied />;

  let config;
  try {
    config = await getConfig();
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Pengaturan Kios</h2>
        <p className="text-sm text-muted-foreground">
          Konfigurasi perilaku bot kios. Perubahan langsung berlaku untuk bot &amp; dashboard.
        </p>
      </div>
      <PengaturanForm config={config} />
    </div>
  );
}
