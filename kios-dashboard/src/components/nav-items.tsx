import { LayoutDashboard, Package, Receipt, BarChart3 } from "lucide-react";

export interface NavItem {
  href: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
}

export const NAV_ITEMS: NavItem[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/produk", label: "Produk & Stok", icon: Package },
  { href: "/penjualan", label: "Penjualan", icon: Receipt },
  { href: "/laporan", label: "Laporan", icon: BarChart3 },
];
