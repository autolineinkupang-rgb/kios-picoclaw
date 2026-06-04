import type { Pesanan } from "./types";
import { formatRupiah } from "./format";

/**
 * Normalize an Indonesian phone number to wa.me digits (62…). Accepts inputs
 * like "08xx", "+62 8xx", "8xx", returning "" when there's nothing usable.
 */
export function normalizeWaNumber(raw: string): string {
  let d = (raw || "").replace(/\D/g, "");
  if (!d) return "";
  if (d.startsWith("0")) d = "62" + d.slice(1);
  else if (d.startsWith("8")) d = "62" + d; // bare local mobile, e.g. "81234…"
  return d;
}

/**
 * Returns true when the raw input resolves to a valid Indonesian mobile number
 * in the canonical "62…" format (2-digit country code + 8–13 local digits).
 */
export function isValidWaNumber(raw: string): boolean {
  const n = normalizeWaNumber(raw);
  return /^62\d{8,13}$/.test(n);
}

/** Build a WhatsApp click-to-chat URL. Empty number → recipient-less compose. */
export function waLink(number: string, text: string): string {
  const n = normalizeWaNumber(number);
  const base = n ? `https://wa.me/${n}` : "https://wa.me/";
  return `${base}?text=${encodeURIComponent(text)}`;
}

/** Friendly, human introduction to Kios Cerdas, sent ahead of any message. */
export const KIOS_INTRO =
  "Halo kak! 👋 Terima kasih sudah belanja di *Kios Cerdas* 🛒\n" +
  "Kios kebutuhan harian di Rote Ndao — pesan online, ambil di kios, tanpa ribet. 😊";

/**
 * Receipt message the kios sends TO the buyer (cashier taps the WhatsApp button
 * in the order inbox). Includes the friendly intro and the struk. For QRIS the
 * cashier attaches a *dynamic* QR photo (amount-specific, screenshotted from
 * their app) separately — so the text only tells the buyer the QR follows.
 */
export function buildStrukWa(p: Pesanan): string {
  const lines = p.items
    .map((it) => `• ${it.nama_produk} x${it.qty} — ${formatRupiah(it.subtotal)}`)
    .join("\n");
  let msg = `${KIOS_INTRO}\n\n🧾 *Struk ${p.id}*\n${lines}\nTotal: *${formatRupiah(p.total)}*\n\n`;
  if (p.metode_bayar === "qris") {
    msg +=
      "💳 Pembayaran: *QRIS*\n" +
      "QR pembayaran sesuai total ini saya kirim sebagai foto di chat ya kak — " +
      "silakan scan & bayar, lalu kirim bukti transfernya. 🙏\n\n";
  } else {
    msg += "💳 Pembayaran: *Tunai* saat ambil pesanan di kios ya kak.\n\n";
  }
  msg += "Sampai jumpa di kios, terima kasih banyak! 🙏✨";
  return msg;
}

/**
 * Message the buyer sends to the online cashier to request a *dynamic* QRIS QR
 * (amount baked in) for their current cart. Used from the storefront checkout
 * when QRIS is selected, as an alternative to scanning the static QR.
 */
export function buildQrisDinamisWa(
  lines: { nama: string; qty: number; subtotal: number }[],
  total: number,
): string {
  const items = lines
    .map((l) => `• ${l.nama} x${l.qty} — ${formatRupiah(l.subtotal)}`)
    .join("\n");
  return (
    "Halo kak kasir *Kios Cerdas*! 👋 Saya mau bayar pakai *QRIS* untuk pesanan:\n\n" +
    `${items}\nTotal: *${formatRupiah(total)}*\n\n` +
    "Boleh minta QR dinamisnya (sesuai total) ya kak? 🙏"
  );
}

export interface OrderWaInput {
  id: string;
  total: number;
  metode: string;
  lines: { nama: string; qty: number; subtotal: number }[];
}

/**
 * Confirmation message the buyer sends TO the kios (storefront success modal).
 * Written from the buyer's point of view since the buyer is the sender.
 */
export function buildOrderWa(o: OrderWaInput): string {
  const items = o.lines
    .map((l) => `• ${l.nama} x${l.qty} — ${formatRupiah(l.subtotal)}`)
    .join("\n");
  let msg =
    "Halo *Kios Cerdas*! 👋 Saya mau konfirmasi pesanan ya kak:\n\n" +
    `🧾 ${o.id}\n${items}\nTotal: *${formatRupiah(o.total)}*\n\n`;
  msg +=
    o.metode === "qris"
      ? "Metode: *QRIS* — saya bayar dulu lalu kirim buktinya ya 🙏"
      : "Metode: *Tunai* saat ambil di kios 🙏";
  return msg;
}
