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

  // Telegram expects the bare bot username: no leading "@", no spaces, no URL.
  // A wrong value here makes the widget render "username invalid".
  const username = (botUsername ?? "")
    .trim()
    .replace(/^@+/, "")
    .replace(/^https?:\/\/t\.me\//i, "");

  useEffect(() => {
    setMounted(true);
    if (!ref.current || !username) return;
    const container = ref.current;
    container.innerHTML = "";

    const authUrl = new URL("/api/auth/telegram", window.location.origin);
    if (next) authUrl.searchParams.set("next", next);

    const script = document.createElement("script");
    script.src = "https://telegram.org/js/telegram-widget.js?22";
    script.async = true;
    script.setAttribute("data-telegram-login", username);
    script.setAttribute("data-size", "large");
    script.setAttribute("data-radius", "8");
    script.setAttribute("data-auth-url", authUrl.toString());
    // Login only needs identity verification — we never DM the user from here,
    // so don't request write access (keeps the consent step minimal).
    container.appendChild(script);
  }, [username, next]);

  if (mounted && !username) {
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
