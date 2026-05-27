import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import {
  SESSION_COOKIE,
  SESSION_TTL_SECONDS,
  createSessionToken,
  resolveDashboardRole,
  verifyTelegramAuth,
} from "@/lib/auth";

// Telegram Login Widget (data-auth-url) redirects the browser here with the
// signed auth fields as query params. We verify, mint a session cookie, and
// bounce to the dashboard. Runs on the Node runtime (uses node:crypto).
export const runtime = "nodejs";

function loginError(req: NextRequest, code: string) {
  const url = req.nextUrl.clone();
  url.pathname = "/login";
  url.search = "";
  url.searchParams.set("error", code);
  return NextResponse.redirect(url);
}

export async function GET(req: NextRequest) {
  const params: Record<string, string> = {};
  req.nextUrl.searchParams.forEach((value, key) => {
    params[key] = value;
  });

  let verified;
  try {
    verified = verifyTelegramAuth(params);
  } catch {
    return loginError(req, "config");
  }
  if (!verified) return loginError(req, "invalid");

  const role = await resolveDashboardRole(verified.id);
  if (!role) return loginError(req, "forbidden");

  const nama =
    [verified.first_name, verified.last_name].filter(Boolean).join(" ").trim() ||
    verified.username ||
    `user-${verified.id}`;

  let token: string;
  try {
    token = await createSessionToken({
      id: verified.id,
      nama,
      username: verified.username ?? "",
      photo_url: verified.photo_url ?? "",
      role,
    });
  } catch {
    return loginError(req, "config");
  }

  const next = params.next && params.next.startsWith("/") ? params.next : "/dashboard";
  const dest = req.nextUrl.clone();
  dest.pathname = next.split("?")[0] || "/dashboard";
  dest.search = next.includes("?") ? next.slice(next.indexOf("?")) : "";

  const res = NextResponse.redirect(dest);
  res.cookies.set(SESSION_COOKIE, token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: SESSION_TTL_SECONDS,
  });
  return res;
}
