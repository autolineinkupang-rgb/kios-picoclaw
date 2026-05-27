"use client";

import { useState, useTransition } from "react";
import { CheckCircle2, Loader2, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { KiosConfig } from "@/lib/types";
import { saveConfigAction, type ActionResult } from "@/app/(app)/pengaturan/actions";

function Toggle({
  checked,
  onChange,
  label,
  description,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
  description: string;
}) {
  return (
    <div className="flex items-center justify-between gap-4 py-3">
      <div className="space-y-0.5">
        <p className="text-sm font-medium">{label}</p>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={label}
        onClick={() => onChange(!checked)}
        className={cn(
          "relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background focus-visible:outline-none",
          checked ? "bg-accent" : "bg-muted-foreground/30",
        )}
      >
        <span
          className={cn(
            "inline-block size-5 transform rounded-full bg-white shadow transition-transform",
            checked ? "translate-x-[22px]" : "translate-x-0.5",
          )}
        />
      </button>
    </div>
  );
}

export function PengaturanForm({ config }: { config: KiosConfig }) {
  const [cfg, setCfg] = useState<KiosConfig>(config);
  const [toast, setToast] = useState<ActionResult | null>(null);
  const [pending, start] = useTransition();

  function save() {
    start(async () => {
      const r = await saveConfigAction(cfg);
      setToast(r);
      window.setTimeout(() => setToast(null), 4000);
    });
  }

  return (
    <div className="max-w-2xl space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Belajar Otomatis</CardTitle>
        </CardHeader>
        <CardContent className="divide-y">
          <Toggle
            checked={cfg.auto_learn_enabled}
            onChange={(v) => setCfg({ ...cfg, auto_learn_enabled: v })}
            label="Aktifkan belajar otomatis"
            description="Bot mencatat alias, shortcut, pola, & kebiasaan dari interaksi."
          />
          <div className="space-y-1.5 py-3">
            <Label htmlFor="learn_model">Model AI untuk pembelajaran</Label>
            <Input
              id="learn_model"
              value={cfg.learn_model}
              onChange={(e) => setCfg({ ...cfg, learn_model: e.target.value })}
              placeholder="kosongkan = ikuti routing default"
              className="font-mono"
            />
            <p className="text-xs text-muted-foreground">
              Mis. <code className="font-mono">groq/llama-3.3-70b</code>. Kosong = otomatis.
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Notifikasi Stok Menipis</CardTitle>
        </CardHeader>
        <CardContent className="divide-y">
          <Toggle
            checked={cfg.notif_enabled}
            onChange={(v) => setCfg({ ...cfg, notif_enabled: v })}
            label="Kirim notifikasi otomatis ke owner"
            description="Peringatan harian untuk produk yang stoknya menipis."
          />
          <div className="space-y-1.5 py-3">
            <Label htmlFor="notif_jam">Jam kirim (WITA)</Label>
            <Select
              id="notif_jam"
              value={cfg.notif_jam}
              onChange={(e) => setCfg({ ...cfg, notif_jam: e.target.value })}
              className="w-32"
              disabled={!cfg.notif_enabled}
            >
              {Array.from({ length: 24 }, (_, h) => String(h).padStart(2, "0")).map((h) => (
                <option key={h} value={h}>
                  {h}:00
                </option>
              ))}
            </Select>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Pembayaran QRIS</CardTitle>
        </CardHeader>
        <CardContent className="divide-y">
          <Toggle
            checked={cfg.qris_enabled}
            onChange={(v) => setCfg({ ...cfg, qris_enabled: v })}
            label="Tampilkan opsi bayar QRIS"
            description="Pembeli di toko & pelanggan via /qris bisa scan QR untuk membayar."
          />
          <div className="space-y-1.5 py-3">
            <Label htmlFor="qris_nama">Nama merchant</Label>
            <Input
              id="qris_nama"
              value={cfg.qris_nama}
              onChange={(e) => setCfg({ ...cfg, qris_nama: e.target.value })}
              placeholder="mis. Kios Cerdas"
              disabled={!cfg.qris_enabled}
            />
          </div>
          <div className="space-y-1.5 py-3">
            <Label htmlFor="qris_image_url">URL gambar QR (QRIS statis)</Label>
            <Input
              id="qris_image_url"
              value={cfg.qris_image_url}
              onChange={(e) => setCfg({ ...cfg, qris_image_url: e.target.value })}
              placeholder="https://…/qris.png"
              className="font-mono"
              inputMode="url"
              disabled={!cfg.qris_enabled}
            />
            <p className="text-xs text-muted-foreground">
              Foto/unggah QR statis dari bank/e-wallet kamu ke layanan gambar, lalu tempel
              tautannya di sini. Pembeli akan memindainya untuk membayar.
            </p>
            {cfg.qris_enabled && cfg.qris_image_url && (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={cfg.qris_image_url}
                alt="Pratinjau QRIS"
                className="mt-2 size-36 rounded-lg border object-contain p-1"
              />
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Kontak WhatsApp</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-1.5 py-1">
            <Label htmlFor="wa_number">Nomor WhatsApp kios</Label>
            <Input
              id="wa_number"
              value={cfg.wa_number}
              onChange={(e) => setCfg({ ...cfg, wa_number: e.target.value })}
              placeholder="mis. 08123456789"
              inputMode="tel"
              className="font-mono"
            />
            <p className="text-xs text-muted-foreground">
              Dipakai tombol &quot;Konfirmasi via WhatsApp&quot; di toko pembeli. Pembeli akan
              langsung diarahkan chat ke nomor ini berisi ringkasan pesanan. Kasir juga bisa
              kirim struk + QRIS ke nomor WhatsApp pembeli dari halaman Pesanan.
            </p>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <Button variant="accent" size="md" onClick={save} disabled={pending}>
          {pending && <Loader2 className="size-4 animate-spin" />}
          Simpan Pengaturan
        </Button>
        {toast && (
          <span
            role="status"
            aria-live="polite"
            className={cn(
              "flex items-center gap-1.5 text-sm",
              toast.ok ? "text-success" : "text-destructive",
            )}
          >
            {toast.ok ? <CheckCircle2 className="size-4" /> : <TriangleAlert className="size-4" />}
            {toast.ok ? toast.message : toast.error}
          </span>
        )}
      </div>
    </div>
  );
}
