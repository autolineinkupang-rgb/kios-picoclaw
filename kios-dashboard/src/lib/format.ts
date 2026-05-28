import type { Periode, Transaksi } from "./types";

/** Format supplier ID agar selaras dengan Go: SUP-001, SUP-042, SUP-123. */
export function formatSuplierId(n: number): string {
  return "SUP-" + String(n).padStart(3, "0");
}

const WITA_OFFSET_MS = 8 * 60 * 60 * 1000;

/** Current time shifted to WITA (UTC+8), matching the Go bot's NowWITA(). */
export function nowWITA(): Date {
  const now = new Date();
  return new Date(now.getTime() + WITA_OFFSET_MS + now.getTimezoneOffset() * 60 * 1000);
}

/** "2026-05-27" in WITA. */
export function todayWITA(): string {
  return formatDateISO(nowWITA());
}

/** "15:04:05" in WITA, matching the Go bot's time format. */
export function timeWITA(): string {
  const d = nowWITA();
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function pad(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}

export function formatDateISO(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

/** Format an integer as Indonesian Rupiah, e.g. 15000 -> "Rp 15.000". */
export function formatRupiah(value: number): string {
  const n = Math.round(value || 0);
  const neg = n < 0;
  const digits = Math.abs(n).toString();
  let out = "";
  for (let i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 === 0) out += ".";
    out += digits[i];
  }
  return `${neg ? "-" : ""}Rp ${out}`;
}

/** Compact Rupiah for tight chart axes, e.g. 1500000 -> "Rp 1,5jt". */
export function formatRupiahCompact(value: number): string {
  const n = Math.round(value || 0);
  const abs = Math.abs(n);
  const sign = n < 0 ? "-" : "";
  if (abs >= 1_000_000) return `${sign}Rp ${trimZero(abs / 1_000_000)}jt`;
  if (abs >= 1_000) return `${sign}Rp ${trimZero(abs / 1_000)}rb`;
  return `${sign}Rp ${abs}`;
}

function trimZero(v: number): string {
  return v.toFixed(1).replace(/\.0$/, "").replace(".", ",");
}

export function formatNumber(value: number): string {
  return new Intl.NumberFormat("id-ID").format(Math.round(value || 0));
}

/** "2026-05-27" -> "27 Mei 2026". Returns the input unchanged if unparseable. */
const BULAN = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "Mei",
  "Jun",
  "Jul",
  "Agu",
  "Sep",
  "Okt",
  "Nov",
  "Des",
];

export function formatTanggal(iso: string): string {
  if (!iso) return "-";
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso);
  if (!m) return iso;
  const [, y, mo, d] = m;
  return `${Number(d)} ${BULAN[Number(mo) - 1] ?? mo} ${y}`;
}

export function periodeLabel(p: Periode): string {
  switch (p) {
    case "minggu":
      return "7 Hari Terakhir";
    case "bulan":
      return "Bulan Ini";
    default:
      return "Hari Ini";
  }
}

/**
 * Filter transactions by period, mirroring LaporanTool.txPeriode in Go:
 *  - hari_ini: tanggal == today
 *  - minggu:   tanggal >= today-6
 *  - bulan:    tanggal starts with current YYYY-MM
 */
export function filterPeriode(txs: Transaksi[], periode: Periode): Transaksi[] {
  const now = nowWITA();
  if (periode === "minggu") {
    const cutoff = new Date(now);
    cutoff.setDate(cutoff.getDate() - 6);
    const cut = formatDateISO(cutoff);
    return txs.filter((t) => t.tanggal >= cut);
  }
  if (periode === "bulan") {
    const prefix = `${now.getFullYear()}-${pad(now.getMonth() + 1)}`;
    return txs.filter((t) => t.tanggal.startsWith(prefix));
  }
  const today = formatDateISO(now);
  return txs.filter((t) => t.tanggal === today);
}
