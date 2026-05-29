"use client";

import { useRef, useState, useTransition } from "react";
import { Bot, CheckCircle2, Clock, ImagePlus, Loader2, MapPin, Store, TriangleAlert, Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { fileToCompressedDataUrl } from "@/lib/image-upload";
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
  const [uploading, setUploading] = useState(false);
  const [uploadingBanner, setUploadingBanner] = useState(false);
  const [pending, start] = useTransition();
  const qrisFileRef = useRef<HTMLInputElement>(null);
  const bannerFileRef = useRef<HTMLInputElement>(null);

  async function onPickQris(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setUploading(true);
    try {
      const dataUrl = await fileToCompressedDataUrl(file, { maxDim: 800, quality: 0.85 });
      setCfg((c) => ({ ...c, qris_image_url: dataUrl }));
    } catch (err) {
      setToast({ ok: false, error: err instanceof Error ? err.message : "Gagal memproses gambar." });
      window.setTimeout(() => setToast(null), 4000);
    } finally {
      setUploading(false);
    }
  }

  async function onPickBanner(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setUploadingBanner(true);
    try {
      const dataUrl = await fileToCompressedDataUrl(file, { maxDim: 1200, quality: 0.85 });
      setCfg((c) => ({ ...c, banner_image_url: dataUrl }));
    } catch (err) {
      setToast({ ok: false, error: err instanceof Error ? err.message : "Gagal memproses gambar." });
      window.setTimeout(() => setToast(null), 4000);
    } finally {
      setUploadingBanner(false);
    }
  }

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
          <CardTitle className="flex items-center gap-2">
            <Bot className="size-4" /> Model AI Utama
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="model_utama">Model untuk semua respons bot</Label>
            <Select
              id="model_utama"
              value={cfg.model_utama ?? ""}
              onChange={(e) => setCfg({ ...cfg, model_utama: e.target.value })}
            >
              <option value="">— Routing default (dari config.json) —</option>
              <optgroup label="Groq">
                <option value="groq/llama-3.3-70b-versatile">Llama 3.3 70B Versatile ⭐ (cepat &amp; pintar)</option>
                <option value="groq/llama3-8b-8192">Llama 3 8B (sangat cepat)</option>
                <option value="groq/mixtral-8x7b-32768">Mixtral 8x7B (konteks panjang)</option>
              </optgroup>
              <optgroup label="Google Gemini">
                <option value="gemini/gemini-2.0-flash-lite">Gemini 2.0 Flash Lite (tercepat)</option>
                <option value="gemini/gemini-2.0-flash">Gemini 2.0 Flash (seimbang)</option>
                <option value="gemini/gemini-1.5-pro">Gemini 1.5 Pro (paling canggih)</option>
              </optgroup>
              <optgroup label="Anthropic">
                <option value="anthropic/claude-haiku-4-5-20251001">Claude Haiku 4.5 (cepat &amp; hemat)</option>
                <option value="anthropic/claude-sonnet-4-6">Claude Sonnet 4.6 (pintar &amp; seimbang)</option>
              </optgroup>
            </Select>
            <p className="text-xs text-muted-foreground">
              Pilih model yang tersedia sesuai API key di Railway. Kosong = ikuti routing default.
              Bisa juga ketik <code className="font-mono">/model</code> di Telegram untuk melihat pilihan.
            </p>
          </div>
          {cfg.model_utama && (
            <div className="flex items-center gap-2 rounded-lg border border-accent/30 bg-accent/5 px-3 py-2 text-xs text-accent">
              <Bot className="size-3.5 shrink-0" />
              Bot menggunakan <span className="font-mono font-medium">{cfg.model_utama}</span> untuk semua respons.
            </div>
          )}
        </CardContent>
      </Card>

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
            <Label htmlFor="qris_image_url">Gambar QR (QRIS statis)</Label>
            <input
              ref={qrisFileRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={onPickQris}
            />
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => qrisFileRef.current?.click()}
                disabled={!cfg.qris_enabled || uploading}
              >
                {uploading ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
                {cfg.qris_image_url ? "Ganti gambar QR" : "Unggah gambar QR"}
              </Button>
              {cfg.qris_image_url && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  disabled={!cfg.qris_enabled}
                  onClick={() => setCfg({ ...cfg, qris_image_url: "" })}
                >
                  Hapus
                </Button>
              )}
            </div>
            <Input
              id="qris_image_url"
              value={cfg.qris_image_url.startsWith("data:") ? "" : cfg.qris_image_url}
              onChange={(e) => setCfg({ ...cfg, qris_image_url: e.target.value })}
              placeholder="atau tempel URL gambar QR…"
              className="font-mono"
              inputMode="url"
              disabled={!cfg.qris_enabled || cfg.qris_image_url.startsWith("data:")}
            />
            <p className="text-xs text-muted-foreground">
              Unggah foto QR statis dari bank/e-wallet kamu (otomatis dikecilkan) atau tempel
              tautannya. Pembeli akan memindainya untuk membayar.
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
          <CardTitle className="flex items-center gap-2">
            <Store className="size-4" /> Tampilan Toko Publik
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 divide-y">
          <div className="space-y-3 pt-1">
            <div className="space-y-1.5">
              <Label htmlFor="nama_toko">Nama toko</Label>
              <Input
                id="nama_toko"
                value={cfg.nama_toko ?? ""}
                onChange={(e) => setCfg({ ...cfg, nama_toko: e.target.value })}
                placeholder="mis. Kios Maju Jaya"
                maxLength={60}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="deskripsi_toko">Deskripsi / tagline</Label>
              <Input
                id="deskripsi_toko"
                value={cfg.deskripsi_toko ?? ""}
                onChange={(e) => setCfg({ ...cfg, deskripsi_toko: e.target.value })}
                placeholder="mis. Pesan online, ambil di kios."
                maxLength={200}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="lokasi_toko" className="flex items-center gap-1">
                <MapPin className="size-3.5" /> Lokasi / desa
              </Label>
              <Input
                id="lokasi_toko"
                value={cfg.lokasi_toko ?? ""}
                onChange={(e) => setCfg({ ...cfg, lokasi_toko: e.target.value })}
                placeholder="mis. Rote Ndao"
                maxLength={80}
              />
            </div>
          </div>

          <div className="space-y-1.5 pt-4">
            <Label className="flex items-center gap-1">
              <Clock className="size-3.5" /> Jam operasional (WITA)
            </Label>
            <div className="flex items-center gap-2">
              <Input
                id="jam_buka"
                type="time"
                value={cfg.jam_buka ?? "08:00"}
                onChange={(e) => setCfg({ ...cfg, jam_buka: e.target.value })}
                className="w-36 font-mono"
                aria-label="Jam buka"
              />
              <span className="text-sm text-muted-foreground">–</span>
              <Input
                id="jam_tutup"
                type="time"
                value={cfg.jam_tutup ?? "21:00"}
                onChange={(e) => setCfg({ ...cfg, jam_tutup: e.target.value })}
                className="w-36 font-mono"
                aria-label="Jam tutup"
              />
            </div>
            <p className="text-xs text-muted-foreground">Ditampilkan di header toko publik.</p>
          </div>

          <div className="space-y-1.5 pt-4">
            <Label htmlFor="banner_image_url" className="flex items-center gap-1">
              <ImagePlus className="size-3.5" /> Gambar banner toko
            </Label>
            <input
              ref={bannerFileRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={onPickBanner}
            />
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => bannerFileRef.current?.click()}
                disabled={uploadingBanner}
              >
                {uploadingBanner ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
                {cfg.banner_image_url ? "Ganti banner" : "Unggah banner"}
              </Button>
              {cfg.banner_image_url && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setCfg({ ...cfg, banner_image_url: "" })}
                >
                  Hapus
                </Button>
              )}
            </div>
            <Input
              id="banner_image_url"
              value={cfg.banner_image_url?.startsWith("data:") ? "" : (cfg.banner_image_url ?? "")}
              onChange={(e) => setCfg({ ...cfg, banner_image_url: e.target.value })}
              placeholder="atau tempel URL gambar banner…"
              className="font-mono"
              inputMode="url"
              disabled={cfg.banner_image_url?.startsWith("data:")}
            />
            {cfg.banner_image_url && (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={cfg.banner_image_url}
                alt="Pratinjau banner"
                className="mt-2 h-24 w-full rounded-lg border object-cover"
              />
            )}
            <p className="text-xs text-muted-foreground">
              Ditampilkan di bagian atas halaman toko publik. Rasio 3:1 ideal (mis. 1200×400px).
            </p>
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
