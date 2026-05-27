import { NextResponse } from "next/server";
import { getAllPesanan } from "@/lib/kios";

// Lightweight health endpoint the bot's /web command pings.
export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET() {
  let redis = false;
  let pesananPending = 0;
  try {
    const orders = await getAllPesanan();
    redis = true;
    pesananPending = orders.filter((o) => o.status === "pending").length;
  } catch {
    redis = false;
  }
  return NextResponse.json({
    ok: true,
    time: new Date().toISOString(),
    redis,
    pesanan_pending: pesananPending,
  });
}
