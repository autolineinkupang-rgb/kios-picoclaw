"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown, LogOut } from "lucide-react";
import type { SessionUser } from "@/lib/types";

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

export function UserMenu({ user }: { user: SessionUser }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        className="flex cursor-pointer items-center gap-2 rounded-lg border border-input py-1 pr-2 pl-1 transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        <span className="flex size-7 items-center justify-center overflow-hidden rounded-full bg-primary text-xs font-semibold text-primary-foreground">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          {user.photo_url ? (
            <img src={user.photo_url} alt="" className="size-full object-cover" />
          ) : (
            initials(user.nama)
          )}
        </span>
        <span className="hidden max-w-[120px] truncate text-sm font-medium sm:block">
          {user.nama}
        </span>
        <ChevronDown className="size-4 text-muted-foreground" />
      </button>

      {open && (
        <div
          role="menu"
          className="absolute right-0 z-50 mt-2 w-56 overflow-hidden rounded-xl border bg-popover text-popover-foreground shadow-lg"
        >
          <div className="border-b px-4 py-3">
            <p className="truncate text-sm font-medium">{user.nama}</p>
            <p className="truncate text-xs text-muted-foreground">
              {user.username ? `@${user.username}` : `ID ${user.id}`}
            </p>
            <span className="mt-2 inline-flex items-center rounded-full bg-accent/15 px-2 py-0.5 text-xs font-medium text-accent capitalize">
              {user.role}
            </span>
          </div>
          <form action="/api/auth/logout" method="post">
            <button
              type="submit"
              role="menuitem"
              className="flex w-full cursor-pointer items-center gap-2 px-4 py-2.5 text-sm text-destructive transition-colors hover:bg-destructive/10"
            >
              <LogOut className="size-4" />
              Keluar
            </button>
          </form>
        </div>
      )}
    </div>
  );
}
