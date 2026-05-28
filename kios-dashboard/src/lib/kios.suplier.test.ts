import test from "node:test";
import assert from "node:assert/strict";
import { formatSuplierId } from "./format.ts";

test("formatSuplierId pads to SUP-NNN like Go", () => {
  assert.equal(formatSuplierId(1), "SUP-001");
  assert.equal(formatSuplierId(42), "SUP-042");
  assert.equal(formatSuplierId(123), "SUP-123");
});
