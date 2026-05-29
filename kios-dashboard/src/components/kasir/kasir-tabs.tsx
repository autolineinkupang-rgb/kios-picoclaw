"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";
import type { Produk, Shift } from "@/lib/types";
import { KasirForm } from "./kasir-form";
import { ShiftTab } from "./shift-tab";

type Tab = "transaksi" | "shift";

interface Props {
  produk: Produk[];
  shift: Shift | null;
  shiftHistory: Shift[];
}

export function KasirTabs({ produk, shift, shiftHistory }: Props) {
  const [tab, setTab] = useState<Tab>("transaksi");

  return (
    <div className="space-y-4">
      {/* Tab bar */}
      <div className="flex border-b">
        {(["transaksi", "shift"] as Tab[]).map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setTab(t)}
            className={cn(
              "-mb-px border-b-2 px-4 py-2 text-sm font-medium capitalize transition-colors",
              tab === t
                ? "border-accent text-accent"
                : "border-transparent text-muted-foreground hover:text-foreground",
            )}
          >
            {t === "transaksi" ? "Transaksi" : "Shift"}
          </button>
        ))}
      </div>

      {tab === "transaksi" && <KasirForm produk={produk} />}
      {tab === "shift" && <ShiftTab shift={shift} history={shiftHistory} />}
    </div>
  );
}
