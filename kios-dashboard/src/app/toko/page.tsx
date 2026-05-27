import type { Metadata } from "next";
import { StorefrontView } from "@/components/toko/storefront-view";

export const metadata: Metadata = {
  title: "Toko Kios Cerdas — Belanja Online",
  description: "Belanja produk kios dengan mudah, tanpa perlu login.",
};
export const dynamic = "force-dynamic";

export default function TokoPage() {
  return <StorefrontView />;
}
