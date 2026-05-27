import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { Store, TriangleAlert } from "lucide-react";
import { getSession } from "@/lib/auth";
import { TelegramLoginButton } from "@/components/telegram-login-button";

export const metadata: Metadata = { title: "Masuk" };

const ERRORS: Record<string, string> = {
  invalid: "Verifikasi login Telegram gagal. Silakan coba lagi.",
  forbidden: "Akun Telegram kamu belum punya akses ke dashboard. Hubungi pemilik kios.",
  config: "Konfigurasi server belum lengkap (token bot / secret). Hubungi admin.",
};

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string; next?: string }>;
}) {
  const session = await getSession();
  if (session) redirect("/dashboard");

  const { error, next } = await searchParams;
  const botUsername = process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME;
  const errMsg = error ? ERRORS[error] ?? "Terjadi kendala saat login." : null;

  return (
    <main className="grid min-h-dvh lg:grid-cols-2">
      {/* Brand panel */}
      <section className="relative hidden flex-col justify-between overflow-hidden bg-primary p-12 text-primary-foreground lg:flex">
        <div
          className="pointer-events-none absolute inset-0 opacity-[0.07]"
          style={{
            backgroundImage:
              "radial-gradient(circle at 1px 1px, currentColor 1px, transparent 0)",
            backgroundSize: "24px 24px",
          }}
          aria-hidden
        />
        <div className="relative flex items-center gap-3">
          <div className="flex size-11 items-center justify-center rounded-xl bg-accent text-accent-foreground">
            <Store className="size-6" aria-hidden />
          </div>
          <span className="text-lg font-semibold">Kios Cerdas</span>
        </div>
        <div className="relative space-y-4">
          <h1 className="text-3xl font-bold leading-tight">
            Kelola kios dari mana saja.
          </h1>
          <p className="max-w-md text-primary-foreground/80">
            Pantau stok, penjualan, dan laba kios secara real-time. Data tersambung
            langsung dengan bot Telegram kios kamu.
          </p>
        </div>
        <p className="relative text-sm text-primary-foreground/60">
          Rote Ndao · Nusa Tenggara Timur
        </p>
      </section>

      {/* Login panel */}
      <section className="flex flex-col items-center justify-center bg-background p-6 sm:p-12">
        <div className="w-full max-w-sm space-y-8">
          <div className="space-y-2 text-center lg:hidden">
            <div className="mx-auto flex size-12 items-center justify-center rounded-xl bg-accent text-accent-foreground">
              <Store className="size-6" aria-hidden />
            </div>
            <h1 className="text-xl font-semibold">Kios Cerdas</h1>
          </div>

          <div className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight">Masuk Dashboard</h2>
            <p className="text-sm text-muted-foreground">
              Gunakan akun Telegram yang sama dengan bot kios untuk masuk.
            </p>
          </div>

          {errMsg && (
            <div
              role="alert"
              className="flex items-start gap-3 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
            >
              <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
              <span>{errMsg}</span>
            </div>
          )}

          <div className="rounded-xl border bg-card p-6">
            <TelegramLoginButton botUsername={botUsername} next={next} />
          </div>

          <p className="text-center text-xs text-muted-foreground">
            Hanya pemilik dan kasir terdaftar yang dapat masuk. Akses diatur lewat
            daftar owner &amp; data pengguna kios.
          </p>
        </div>
      </section>
    </main>
  );
}
