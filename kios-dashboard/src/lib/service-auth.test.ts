import test from "node:test";
import assert from "node:assert/strict";
import crypto from "node:crypto";

import { verifyServiceToken, SERVICE_MAX_AGE_SECONDS } from "./service-auth.ts";

const SECRET = "test-service-secret-0123456789ab";
process.env.KIOS_SERVICE_SECRET = SECRET;

// Build the header the bot would send for a given unix timestamp.
function makeToken(ts: number): string {
  const sig = crypto
    .createHmac("sha256", SECRET)
    .update("kios-service:" + ts)
    .digest("hex");
  return `${ts}.${sig}`;
}

const now = new Date("2026-05-28T06:00:00Z");
const nowSec = Math.floor(now.getTime() / 1000);

test("accepts a valid, fresh token", () => {
  assert.equal(verifyServiceToken(makeToken(nowSec), now), true);
});

test("rejects a token with a tampered signature", () => {
  const [ts] = makeToken(nowSec).split(".");
  const bad = `${ts}.${"0".repeat(64)}`;
  assert.equal(verifyServiceToken(bad, now), false);
});

test("rejects a signature of the wrong length", () => {
  const [ts] = makeToken(nowSec).split(".");
  assert.equal(verifyServiceToken(`${ts}.abcd`, now), false);
});

test("rejects a token older than the freshness window", () => {
  const stale = makeToken(nowSec - SERVICE_MAX_AGE_SECONDS - 5);
  assert.equal(verifyServiceToken(stale, now), false);
});

test("rejects a token too far in the future", () => {
  const future = makeToken(nowSec + SERVICE_MAX_AGE_SECONDS + 5);
  assert.equal(verifyServiceToken(future, now), false);
});

test("accepts a token at the edge of the freshness window", () => {
  const edge = makeToken(nowSec - SERVICE_MAX_AGE_SECONDS);
  assert.equal(verifyServiceToken(edge, now), true);
});

test("rejects a missing or malformed header", () => {
  assert.equal(verifyServiceToken(null, now), false);
  assert.equal(verifyServiceToken("", now), false);
  assert.equal(verifyServiceToken("no-dot-here", now), false);
  assert.equal(verifyServiceToken(`${nowSec}.`, now), false);
  assert.equal(verifyServiceToken(`notanumber.${"a".repeat(64)}`, now), false);
});
