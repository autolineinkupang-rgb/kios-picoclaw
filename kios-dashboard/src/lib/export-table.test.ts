import test from "node:test";
import assert from "node:assert/strict";
import { toCsvString } from "./export-table.ts";

test("toCsvString: basic headers and rows", () => {
  const result = toCsvString(["id", "nama"], [["001", "Beras"], ["002", "Gula"]]);
  assert.equal(result, "id,nama\n001,Beras\n002,Gula");
});

test("toCsvString: escapes commas and quotes", () => {
  const result = toCsvString(["nama", "catatan"], [['Toko "ABC"', "jl. merdeka, no 1"]]);
  assert.equal(result, 'nama,catatan\n"Toko ""ABC""","jl. merdeka, no 1"');
});

test("toCsvString: empty rows returns only header", () => {
  const result = toCsvString(["id", "nama"], []);
  assert.equal(result, "id,nama");
});
