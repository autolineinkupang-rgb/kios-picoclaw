"use client";

import { usePathname } from "next/navigation";
import { NAV_ITEMS } from "./nav-items";

export function PageTitle() {
  const pathname = usePathname();
  const item =
    NAV_ITEMS.find((i) => pathname === i.href || pathname.startsWith(`${i.href}/`)) ??
    NAV_ITEMS[0];
  return <h1 className="text-base font-semibold tracking-tight">{item.label}</h1>;
}
