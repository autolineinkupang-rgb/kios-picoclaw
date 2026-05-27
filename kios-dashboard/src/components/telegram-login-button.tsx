"use client";

import { useEffect, useRef, useState } from "react";

interface Props {
  botUsername: string | undefined;
  next?: string;
}

/**
 * Renders the official Telegram Login Widget. On success Telegram redirects the
 * browser to /api/auth/telegram (data-auth-url) with the signed auth fields,
 * which the route handler verifies and turns into a session cookie.
 */
export function TelegramLoginButton({ botUsername, next }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
    if (!ref.current || !botUsername) return;
    const container = ref.current;
    container.innerHTML = "";

    const authUrl = new URL("/api/auth/telegram", window.location.origin);
    if (next) authUrl.searchParams.set("next", next);

    const script = document.createElement("script");
    script.src = "https://telegram.org/js/telegram-widget.js?22";
    script.async = true;
    script.setAttribute("data-telegram-login", botUsername);
    script.setAttribute("data-size", "large");
    script.setAttribute("data-radius", "8");
    script.setAttribute("data-auth-url", authUrl.toString());
    script.setAttribute("data-request-access", "write");
    container.appendChild(script);
  }, [botUsername, next]);

  if (mounted && !botUsername) {
    return (
      <p className="text-sm text-destructive">
        Konfigurasi <code className="font-mono">NEXT_PUBLIC_TELEGRAM_BOT_USERNAME</code> belum
        di-set.
      </p>
    );
  }

  return (
    <div className="flex min-h-[48px] items-center justify-center" ref={ref} aria-live="polite">
      <span className="text-sm text-muted-foreground">Memuat tombol Telegram…</span>
    </div>
  );
}
