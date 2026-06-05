"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";
import { ImporProdukTab } from "./impor-produk-tab";
import { ImporPiutangTab } from "./impor-piutang-tab";
import { ImporHutangTab } from "./impor-hutang-tab";
import { ImporPenjualanTab } from "./impor-penjualan-tab";
import type { Role } from "@/lib/types";

const TABS = [
  { id: "produk", label: "Produk & Stok" },
  { id: "piutang", label: "Piutang" },
  { id: "hutang", label: "Hutang" },
  { id: "penjualan", label: "Penjualan" },
] as const;
type TabId = (typeof TABS)[number]["id"];

export function ImporView({ role }: { role: Role }) {
  const [tab, setTab] = useState<TabId>("produk");
  const isOwner = role === "owner";

  return (
    <div className="max-w-3xl space-y-4">
      {/* Tab navigation */}
      <div className="flex overflow-x-auto rounded-lg border bg-muted/30 p-1 gap-1">
        {TABS.filter((t) => t.id === "produk" || isOwner).map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => setTab(t.id as TabId)}
            className={cn(
              "flex-shrink-0 rounded-md px-3 py-1.5 text-sm font-medium transition-colors whitespace-nowrap",
              tab === t.id
                ? "bg-background shadow text-foreground"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "produk" && <ImporProdukTab role={role} />}
      {tab === "piutang" && isOwner && <ImporPiutangTab />}
      {tab === "hutang" && isOwner && <ImporHutangTab />}
      {tab === "penjualan" && isOwner && <ImporPenjualanTab />}
    </div>
  );
}
