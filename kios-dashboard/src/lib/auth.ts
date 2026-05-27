import crypto from "node:crypto";
import { cookies } from "next/headers";
import { SignJWT, jwtVerify } from "jose";
import { getUser } from "./kios";
import type { Role, SessionUser } from "./types";

export const SESSION_COOKIE = "kios_session";
const SESSION_TTL_SECONDS = 60 * 60 * 24 * 7; // 7 days
const AUTH_MAX_AGE_SECONDS = 60 * 60 * 24; // reject widget payloads older than 1 day

export interface TelegramAuthData {
  id: string;
  first_name?: string;
  last_name?: string;
  username?: string;
  photo_url?: string;
  auth_date: string;
  hash: string;
}

function secretKey(): Uint8Array {
  const s = process.env.SESSION_SECRET;
  if (!s || s.length < 16) {
    throw new Error("SESSION_SECRET belum di-set (minimal 16 karakter). Lihat .env.example.");
  }
  return new TextEncoder().encode(s);
}

/**
 * Verify a Telegram Login Widget payload per
 * https://core.telegram.org/widgets/login#checking-authorization.
 * Returns the verified data, or null if the hash/age check fails.
 */
export function verifyTelegramAuth(
  params: Record<string, string>,
): TelegramAuthData | null {
  const token = process.env.TELEGRAM_BOT_TOKEN;
  if (!token) throw new Error("TELEGRAM_BOT_TOKEN belum di-set. Lihat .env.example.");

  const { hash, ...rest } = params;
  if (!hash || !rest.id || !rest.auth_date) return null;

  const dataCheckString = Object.keys(rest)
    .filter((k) => rest[k] !== undefined && rest[k] !== "")
    .sort()
    .map((k) => `${k}=${rest[k]}`)
    .join("\n");

  const secret = crypto.createHash("sha256").update(token).digest();
  const computed = crypto
    .createHmac("sha256", secret)
    .update(dataCheckString)
    .digest("hex");

  const a = Buffer.from(computed, "hex");
  const b = Buffer.from(hash, "hex");
  if (a.length !== b.length || !crypto.timingSafeEqual(a, b)) return null;

  const authDate = Number(rest.auth_date);
  if (!Number.isFinite(authDate)) return null;
  if (Date.now() / 1000 - authDate > AUTH_MAX_AGE_SECONDS) return null;

  return { ...(rest as Omit<TelegramAuthData, "hash">), hash };
}

function isBootstrapOwner(id: string): boolean {
  const ids = (process.env.KIOS_OWNER_IDS ?? "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
  return ids.includes(id.trim());
}

/**
 * Decide whether a verified Telegram user may access the dashboard and with
 * what role. Mirrors the bot RBAC: KIOS_OWNER_IDS are always owner; otherwise
 * the user must exist and be active in the kios:users hash.
 * Returns null when access is denied.
 */
export async function resolveDashboardRole(id: string): Promise<Role | null> {
  if (isBootstrapOwner(id)) return "owner";
  const user = await getUser(id);
  if (!user || !user.aktif) return null;
  return user.role === "owner" ? "owner" : "kasir";
}

export async function createSessionToken(user: SessionUser): Promise<string> {
  return new SignJWT({
    nama: user.nama,
    username: user.username,
    photo_url: user.photo_url,
    role: user.role,
  })
    .setProtectedHeader({ alg: "HS256" })
    .setSubject(user.id)
    .setIssuedAt()
    .setExpirationTime(`${SESSION_TTL_SECONDS}s`)
    .sign(secretKey());
}

export async function verifySessionToken(
  token: string | undefined,
): Promise<SessionUser | null> {
  if (!token) return null;
  try {
    const { payload } = await jwtVerify(token, secretKey());
    if (!payload.sub) return null;
    return {
      id: payload.sub,
      nama: (payload.nama as string) ?? "",
      username: (payload.username as string) ?? "",
      photo_url: (payload.photo_url as string) ?? "",
      role: (payload.role as Role) ?? "kasir",
    };
  } catch {
    return null;
  }
}

/** Read and verify the current session from cookies (server components). */
export async function getSession(): Promise<SessionUser | null> {
  const jar = await cookies();
  return verifySessionToken(jar.get(SESSION_COOKIE)?.value);
}

export { SESSION_TTL_SECONDS };
