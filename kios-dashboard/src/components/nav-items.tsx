import {
  LayoutDashboard,
  Package,
  Receipt,
  BarChart3,
  ShoppingCart,
  ClipboardList,
  FileUp,
  Users,
  Settings,
  Truck,
  Contact,
  Zap,
} from "lucide-react";
import type { Role } from "@/lib/types";

export interface NavItem {
  href: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  ownerOnly?: boolean;
}

export const NAV_ITEMS: NavItem[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/kasir", label: "Kasir", icon: ShoppingCart },
  { href: "/pesanan", label: "Pesanan", icon: ClipboardList },
  { href: "/pelanggan", label: "Pelanggan", icon: Contact },
  { href: "/produk", label: "Produk & Stok", icon: Package },
  { href: "/produk-sampingan", label: "Produk Sampingan", icon: Zap },
  { href: "/suplier", label: "Supplier", icon: Truck },
  { href: "/impor", label: "Impor Data", icon: FileUp },
  { href: "/penjualan", label: "Penjualan", icon: Receipt },
  { href: "/laporan", label: "Laporan", icon: BarChart3 },
  { href: "/pengguna", label: "Pengguna", icon: Users, ownerOnly: true },
  { href: "/pengaturan", label: "Pengaturan", icon: Settings, ownerOnly: true },
];

/** Nav items visible to the given role (owner sees all; kasir sees non-owner items). */
export function navItemsForRole(role: Role | undefined): NavItem[] {
  if (role === "owner") return NAV_ITEMS;
  return NAV_ITEMS.filter((i) => !i.ownerOnly);
}
