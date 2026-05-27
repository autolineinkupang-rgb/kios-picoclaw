// Client-side image helper: reads a user-picked image File, downscales it with
// a canvas, and returns a compressed JPEG data URL. Storing the data URL keeps
// images in the same Redis records as everything else — no external blob store
// or credentials needed. Compression keeps each image small enough for that.

export const MAX_IMAGE_DATA_URL = 600_000; // ~440 KB of base64 — safety cap

export interface CompressOptions {
  maxDim?: number; // longest edge in px
  quality?: number; // 0..1 JPEG quality
}

/** Resize + compress an image File into a `data:image/jpeg;base64,…` URL. */
export function fileToCompressedDataUrl(
  file: File,
  { maxDim = 600, quality = 0.72 }: CompressOptions = {},
): Promise<string> {
  return new Promise((resolve, reject) => {
    if (!file.type.startsWith("image/")) {
      reject(new Error("File yang dipilih bukan gambar."));
      return;
    }
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(url);
      const scale = Math.min(1, maxDim / Math.max(img.width, img.height));
      const w = Math.max(1, Math.round(img.width * scale));
      const h = Math.max(1, Math.round(img.height * scale));
      const canvas = document.createElement("canvas");
      canvas.width = w;
      canvas.height = h;
      const ctx = canvas.getContext("2d");
      if (!ctx) {
        reject(new Error("Browser tidak mendukung pemrosesan gambar."));
        return;
      }
      ctx.drawImage(img, 0, 0, w, h);
      const dataUrl = canvas.toDataURL("image/jpeg", quality);
      if (dataUrl.length > MAX_IMAGE_DATA_URL) {
        reject(new Error("Gambar terlalu besar. Coba foto dengan ukuran lebih kecil ya."));
        return;
      }
      resolve(dataUrl);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("Gagal membaca gambar. Coba file lain ya."));
    };
    img.src = url;
  });
}
