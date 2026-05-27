"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import {
  CheckCircle2,
  Loader2,
  Plus,
  Power,
  Trash2,
  TriangleAlert,
  UserCog,
  Users,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input, Label, Select } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { EmptyState } from "@/components/ui/empty-state";
import { formatTanggal } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Role, UserKios } from "@/lib/types";
import {
  deleteUserAction,
  saveUserAction,
  toggleUserAktifAction,
  type ActionResult,
} from "@/app/(app)/pengguna/actions";

export function PenggunaManager({
  users,
  currentUserId,
}: {
  users: UserKios[];
  currentUserId: string;
}) {
  const router = useRouter();
  const [dialog, setDialog] = useState<{ mode: "add" | "edit"; user?: UserKios } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<UserKios | null>(null);
  const [toast, setToast] = useState<ActionResult | null>(null);
  const [pending, start] = useTransition();

  function showToast(r: ActionResult) {
    setToast(r);
    window.setTimeout(() => setToast(null), 4000);
  }

  function run(fn: () => Promise<ActionResult>, after?: () => void) {
    start(async () => {
      const r = await fn();
      showToast(r);
      if (r.ok) {
        after?.();
        router.refresh();
      }
    });
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">{users.length} pengguna terdaftar</p>
        <Button variant="accent" size="md" onClick={() => setDialog({ mode: "add" })}>
          <Plus className="size-4" /> Tambah
        </Button>
      </div>

      {users.length === 0 ? (
        <EmptyState
          icon={Users}
          title="Belum ada pengguna"
          description="Tambahkan kasir atau owner berdasarkan Telegram ID mereka."
        />
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <table className="w-full min-w-[640px] text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="p-3 font-medium">Nama</th>
                <th className="p-3 font-medium">Telegram ID</th>
                <th className="p-3 font-medium">Role</th>
                <th className="p-3 font-medium">Status</th>
                <th className="p-3 font-medium">Ditambahkan</th>
                <th className="p-3 text-right font-medium">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => {
                const self = u.phone === currentUserId;
                return (
                  <tr key={u.phone} className="border-b transition-colors last:border-0 hover:bg-muted/40">
                    <td className="p-3 font-medium">
                      {u.nama}
                      {self && <span className="ml-1.5 text-xs text-muted-foreground">(kamu)</span>}
                    </td>
                    <td className="p-3 font-mono text-xs text-muted-foreground">{u.phone}</td>
                    <td className="p-3">
                      <Badge variant={u.role === "owner" ? "default" : "secondary"} className="capitalize">
                        {u.role}
                      </Badge>
                    </td>
                    <td className="p-3">
                      <Badge variant={u.aktif ? "success" : "destructive"}>
                        {u.aktif ? "Aktif" : "Nonaktif"}
                      </Badge>
                    </td>
                    <td className="p-3 text-xs text-muted-foreground">{formatTanggal(u.ditambahkan)}</td>
                    <td className="p-3">
                      <div className="flex justify-end gap-1">
                        <button
                          type="button"
                          onClick={() => setDialog({ mode: "edit", user: u })}
                          aria-label={`Edit ${u.nama}`}
                          className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                        >
                          <UserCog className="size-4" />
                        </button>
                        <button
                          type="button"
                          disabled={self || pending}
                          onClick={() => run(() => toggleUserAktifAction(u.phone))}
                          aria-label={u.aktif ? `Nonaktifkan ${u.nama}` : `Aktifkan ${u.nama}`}
                          className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
                        >
                          <Power className="size-4" />
                        </button>
                        <button
                          type="button"
                          disabled={self || pending}
                          onClick={() => setDeleteTarget(u)}
                          aria-label={`Hapus ${u.nama}`}
                          className="flex size-8 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive disabled:cursor-not-allowed disabled:opacity-40"
                        >
                          <Trash2 className="size-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <Modal
        open={dialog !== null}
        onClose={() => setDialog(null)}
        title={dialog?.mode === "edit" ? "Edit Pengguna" : "Tambah Pengguna"}
        description={dialog?.mode === "edit" ? dialog.user?.nama : "Masukkan Telegram ID & peran pengguna."}
        className="max-w-md"
      >
        {dialog && (
          <UserForm
            user={dialog.user}
            pending={pending}
            onSubmit={(input) => run(() => saveUserAction(input), () => setDialog(null))}
            onCancel={() => setDialog(null)}
          />
        )}
      </Modal>

      <Modal
        open={deleteTarget !== null}
        onClose={() => !pending && setDeleteTarget(null)}
        title="Hapus pengguna?"
        description="Pengguna tidak bisa lagi mengakses kios sampai didaftarkan ulang."
        className="max-w-md"
      >
        <div className="space-y-4">
          <p className="text-sm">
            Yakin hapus <span className="font-medium">{deleteTarget?.nama}</span> (
            {deleteTarget?.phone})?
          </p>
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setDeleteTarget(null)} disabled={pending}>
              Batal
            </Button>
            <Button
              variant="destructive"
              size="sm"
              disabled={pending}
              onClick={() => {
                const t = deleteTarget;
                if (t) run(() => deleteUserAction(t.phone), () => setDeleteTarget(null));
              }}
            >
              {pending && <Loader2 className="size-4 animate-spin" />} Hapus
            </Button>
          </div>
        </div>
      </Modal>

      {toast && (
        <div
          role="status"
          aria-live="polite"
          className={cn(
            "fixed bottom-4 left-1/2 z-[60] flex -translate-x-1/2 items-center gap-2 rounded-lg border bg-card px-4 py-2.5 text-sm shadow-lg",
            toast.ok ? "border-success/30 text-foreground" : "border-destructive/30 text-destructive",
          )}
        >
          {toast.ok ? <CheckCircle2 className="size-4 text-success" /> : <TriangleAlert className="size-4" />}
          {toast.ok ? toast.message : toast.error}
        </div>
      )}
    </div>
  );
}

function UserForm({
  user,
  pending,
  onSubmit,
  onCancel,
}: {
  user?: UserKios;
  pending: boolean;
  onSubmit: (input: { phone: string; nama: string; role: Role }) => void;
  onCancel: () => void;
}) {
  const [phone, setPhone] = useState(user?.phone ?? "");
  const [nama, setNama] = useState(user?.nama ?? "");
  const [role, setRole] = useState<Role>((user?.role as Role) ?? "kasir");
  const isEdit = Boolean(user);

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit({ phone, nama, role });
      }}
      className="space-y-4"
    >
      <div className="space-y-1.5">
        <Label htmlFor="phone">Telegram ID</Label>
        <Input
          id="phone"
          value={phone}
          onChange={(e) => setPhone(e.target.value.replace(/\D/g, ""))}
          inputMode="numeric"
          placeholder="mis. 123456789"
          disabled={isEdit}
          className="font-mono"
          required
        />
        <p className="text-xs text-muted-foreground">
          ID numerik akun Telegram. Suruh user kirim /login atau cek via @userinfobot.
        </p>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="nama">Nama</Label>
        <Input id="nama" value={nama} onChange={(e) => setNama(e.target.value)} required autoFocus={isEdit} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="role">Peran</Label>
        <Select id="role" value={role} onChange={(e) => setRole(e.target.value as Role)}>
          <option value="kasir">Kasir (jual & lihat stok)</option>
          <option value="owner">Owner (akses penuh)</option>
        </Select>
      </div>
      <div className="flex justify-end gap-2 pt-2">
        <Button type="button" variant="outline" size="sm" onClick={onCancel} disabled={pending}>
          Batal
        </Button>
        <Button type="submit" variant="accent" size="sm" disabled={pending || !phone || !nama}>
          {pending && <Loader2 className="size-4 animate-spin" />}
          {isEdit ? "Simpan" : "Tambah"}
        </Button>
      </div>
    </form>
  );
}
