// Maps a product to a display image. When a product has no uploaded image, we
// fall back to a category placeholder SVG shipped in public/produk/ (committed
// to the repo, served by Vercel's CDN — no Redis/server storage involved).

const KNOWN = new Set(["sembako", "mie", "minuman", "rokok", "gas", "kebutuhan", "snack"]);

/** Resolve a category name to a placeholder SVG path under /produk. */
export function categoryImage(kategori: string): string {
  const k = (kategori || "").toLowerCase();
  const has = (...names: string[]) => names.some((n) => k.includes(n));

  let file = "umum";
  if (KNOWN.has(k.trim())) file = k.trim();
  else if (has("sembako", "beras", "gula", "minyak", "tepung", "telur")) file = "sembako";
  else if (has("mie", "indomie", "instan")) file = "mie";
  else if (has("minum", "teh", "kopi", "air ", "soda", "jus")) file = "minuman";
  else if (has("rokok", "tembakau")) file = "rokok";
  else if (has("gas", "lpg", "elpiji")) file = "gas";
  else if (has("snack", "cemilan", "camilan", "kerupuk", "biskuit", "keripik")) file = "snack";
  else if (has("kebutuhan", "sabun", "shampo", "sampo", "pasta gigi", "deterjen", "rumah")) file = "kebutuhan";

  return `/produk/${file}.svg`;
}

/** The image to show for a product: its own image, else a category placeholder. */
export function produkImage(p: { image_url?: string; kategori?: string }): string {
  const img = (p.image_url || "").trim();
  return img || categoryImage(p.kategori || "");
}
