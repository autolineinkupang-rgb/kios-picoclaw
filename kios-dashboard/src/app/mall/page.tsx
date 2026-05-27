import { redirect } from "next/navigation";

// Storefront consolidated into /toko. Keep /mall working as a redirect so any
// previously shared links don't break.
export default function MallPage() {
  redirect("/toko");
}
