import crypto from "node:crypto";

// The bot signs with: sig = HMAC_SHA256(KIOS_SERVICE_SECRET, "kios-service:" + ts)
// and sends the header value "<ts>.<hexsig>" (ts = unix seconds).
const SERVICE_MESSAGE_PREFIX = "kios-service:";

// How much clock skew / replay window to tolerate, in seconds. The bot and the
// dashboard run on different hosts, so their clocks won't match exactly.
export const SERVICE_MAX_AGE_SECONDS = 60;

function serviceSecret(): string {
  const s = process.env.KIOS_SERVICE_SECRET;
  if (!s || s.length < 16) {
    throw new Error(
      "KIOS_SERVICE_SECRET belum di-set (minimal 16 karakter). Lihat .env.example.",
    );
  }
  return s;
}

/**
 * Verify the X-Kios-Service header the bot attaches to service-to-service
 * requests (e.g. GET /api/summary). Returns true only when the signature is
 * valid AND the timestamp is fresh enough to reject replays.
 *
 * Header format: "<ts>.<hexsig>" where
 *   sig = HMAC_SHA256(KIOS_SERVICE_SECRET, "kios-service:" + ts)
 *
 * @param header the raw X-Kios-Service header value (or null if absent)
 * @param now    injectable clock for testing; defaults to the current time
 */
export function verifyServiceToken(
  header: string | null | undefined,
  now: Date = new Date(),
): boolean {
  if (!header) return false;

  const dot = header.indexOf(".");
  if (dot <= 0) return false;
  const tsRaw = header.slice(0, dot);
  const sigHex = header.slice(dot + 1);
  if (!/^\d+$/.test(tsRaw) || !/^[0-9a-f]+$/i.test(sigHex)) return false;

  // Recompute the signature the bot should have produced for this timestamp.
  const expectedHex = crypto
    .createHmac("sha256", serviceSecret())
    .update(SERVICE_MESSAGE_PREFIX + tsRaw)
    .digest("hex");

  const ts = Number(tsRaw); // unix seconds claimed by the caller
  const nowSec = Math.floor(now.getTime() / 1000);

  // 1. Constant-time signature check. timingSafeEqual throws on length
  //    mismatch, so guard the lengths first (a forged sig of the wrong size
  //    is rejected here without leaking timing).
  const expected = Buffer.from(expectedHex, "hex");
  const got = Buffer.from(sigHex, "hex");
  if (expected.length !== got.length || !crypto.timingSafeEqual(expected, got)) {
    return false;
  }

  // 2. Freshness: reject tokens outside the symmetric skew window. This bounds
  //    how long a captured-but-valid token can be replayed.
  if (Math.abs(nowSec - ts) > SERVICE_MAX_AGE_SECONDS) {
    return false;
  }

  return true;
}
