import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import {
  SESSION_COOKIE,
  SESSION_TTL_SECONDS,
  createSessionToken,
  resolveDashboardRole,
} from "@/lib/auth";
import { bumpLoginAttempts, consumeLoginCode } from "@/lib/kios";

export const runtime = "nodejs";

const MAX_ATTEMPTS = 10;

function back(req: NextRequest, error: string, next?: string) {
  const url = req.nextUrl.clone();
  url.pathname = "/login";
  url.search = "";
  url.searchParams.set("error", error);
  if (next) url.searchParams.set("next", next);
  return NextResponse.redirect(url, { status: 303 });
}

export async function POST(req: NextRequest) {
  const form = await req.formData();
  const code = String(form.get("code") ?? "").replace(/\D/g, "");
  const next = String(form.get("next") ?? "");
  const safeNext = next.startsWith("/") ? next : "/dashboard";

  if (!/^\d{6}$/.test(code)) return back(req, "code_format", next || undefined);

  const ip =
    req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ||
    req.headers.get("x-real-ip") ||
    "unknown";

  try {
    const attempts = await bumpLoginAttempts(ip);
    if (attempts > MAX_ATTEMPTS) return back(req, "too_many", next || undefined);

    const rec = await consumeLoginCode(code);
    if (!rec) return back(req, "code_invalid", next || undefined);

    const role = await resolveDashboardRole(rec.id);
    if (!role) return back(req, "forbidden");

    const token = await createSessionToken({
      id: rec.id,
      nama: rec.nama || `user-${rec.id}`,
      username: "",
      photo_url: "",
      role,
    });

    const dest = req.nextUrl.clone();
    dest.pathname = safeNext.split("?")[0] || "/dashboard";
    dest.search = safeNext.includes("?") ? safeNext.slice(safeNext.indexOf("?")) : "";

    const res = NextResponse.redirect(dest, { status: 303 });
    res.cookies.set(SESSION_COOKIE, token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: SESSION_TTL_SECONDS,
    });
    return res;
  } catch {
    return back(req, "config", next || undefined);
  }
}
