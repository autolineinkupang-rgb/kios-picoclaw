"use client";

import { cn } from "@/lib/utils";
import type { Periode } from "@/lib/types";

const OPTIONS: { value: Periode; label: string }[] = [
  { value: "hari_ini", label: "Hari Ini" },
  { value: "minggu", label: "7 Hari" },
  { value: "bulan", label: "Bulan Ini" },
];

export function PeriodeTabs({
  value,
  onChange,
}: {
  value: Periode;
  onChange: (p: Periode) => void;
}) {
  return (
    <div
      role="tablist"
      aria-label="Pilih periode"
      className="inline-flex rounded-lg border bg-muted/50 p-1"
    >
      {OPTIONS.map((o) => {
        const active = value === o.value;
        return (
          <button
            key={o.value}
            role="tab"
            type="button"
            aria-selected={active}
            onClick={() => onChange(o.value)}
            className={cn(
              "cursor-pointer rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
              active
                ? "bg-card text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
