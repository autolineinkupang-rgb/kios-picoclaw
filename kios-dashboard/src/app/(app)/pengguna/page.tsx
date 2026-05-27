import type { Metadata } from "next";
import { getAllUsers } from "@/lib/kios";
import { getSession } from "@/lib/auth";
import { ConnectionError } from "@/components/connection-error";
import { AccessDenied } from "@/components/access-denied";
import { PenggunaManager } from "@/components/pengguna/pengguna-manager";

export const metadata: Metadata = { title: "Pengguna" };
export const dynamic = "force-dynamic";

export default async function PenggunaPage() {
  const session = await getSession();
  if (session?.role !== "owner") return <AccessDenied />;

  let users;
  try {
    users = await getAllUsers();
  } catch (e) {
    return <ConnectionError message={e instanceof Error ? e.message : String(e)} />;
  }

  users.sort((a, b) => a.nama.localeCompare(b.nama));

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Manajemen Pengguna</h2>
        <p className="text-sm text-muted-foreground">
          Kelola owner &amp; kasir yang boleh mengakses kios (bot &amp; dashboard).
        </p>
      </div>
      <PenggunaManager users={users} currentUserId={session.id} />
    </div>
  );
}
