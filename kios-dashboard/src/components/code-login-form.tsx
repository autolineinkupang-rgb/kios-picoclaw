"use client";

import { useEffect, useRef, useState } from "react";
import { ArrowRight, Send } from "lucide-react";
import { Button } from "@/components/ui/button";

export function CodeLoginForm({
  next,
  initialCode,
  botUsername,
}: {
  next?: string;
  initialCode?: string;
  botUsername?: string;
}) {
  const formRef = useRef<HTMLFormElement>(null);
  const [code, setCode] = useState((initialCode ?? "").replace(/\D/g, "").slice(0, 6));
  const [submitting, setSubmitting] = useState(false);
  const username = (botUsername ?? "").trim().replace(/^@+/, "");

  // Tap-to-login: when the bot link prefills a valid code, submit automatically.
  useEffect(() => {
    if (/^\d{6}$/.test(code) && initialCode) {
      setSubmitting(true);
      formRef.current?.requestSubmit();
    }
    // run once on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="space-y-4">
      <form
        ref={formRef}
        action="/api/auth/code"
        method="post"
        onSubmit={() => setSubmitting(true)}
        className="space-y-3"
      >
        {next && <input type="hidden" name="next" value={next} />}
        <input
          name="code"
          value={code}
          onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
          inputMode="numeric"
          autoComplete="one-time-code"
          pattern="\d{6}"
          placeholder="······"
          aria-label="Kode masuk 6 digit"
          autoFocus
          className="h-14 w-full rounded-lg border border-input bg-background text-center font-mono text-2xl tracking-[0.4em] outline-none transition-colors placeholder:text-muted-foreground/50 focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30"
        />
        <Button
          type="submit"
          variant="accent"
          size="md"
          className="w-full"
          disabled={code.length !== 6 || submitting}
        >
          {submitting ? "Memverifikasi…" : "Masuk"}
          {!submitting && <ArrowRight className="size-4" />}
        </Button>
      </form>

      {username && (
        <a
          href={`https://t.me/${username}`}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center justify-center gap-2 text-sm font-medium text-accent hover:underline"
        >
          <Send className="size-4" />
          Buka bot &amp; ketik /login
        </a>
      )}
    </div>
  );
}
