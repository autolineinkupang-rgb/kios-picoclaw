// Client-side spreadsheet parsing for the import feature.
// Produces rows keyed by normalised header (lowercase, spaces -> underscore).
// CSV is parsed natively; .xlsx/.xls via read-excel-file (dynamic import).

export type TableRow = Record<string, string>;

function normKey(k: string): string {
  return String(k).trim().toLowerCase().replace(/\s+/g, "_");
}

function cellToString(v: unknown): string {
  if (v == null) return "";
  if (v instanceof Date) return v.toISOString().slice(0, 10);
  return String(v).trim();
}

function matrixToRows(matrix: unknown[][]): TableRow[] {
  if (!matrix.length) return [];
  const headers = matrix[0].map((h) => normKey(cellToString(h)));
  const rows: TableRow[] = [];
  for (let i = 1; i < matrix.length; i++) {
    const cells = matrix[i];
    if (!cells || cells.every((c) => cellToString(c) === "")) continue;
    const row: TableRow = {};
    headers.forEach((h, j) => {
      if (h) row[h] = cellToString(cells[j]);
    });
    rows.push(row);
  }
  return rows;
}

// Minimal RFC-4180-ish CSV parser (handles quoted fields, commas, newlines).
function parseCsv(text: string): TableRow[] {
  const records: string[][] = [];
  let field = "";
  let record: string[] = [];
  let inQuotes = false;
  const pushField = () => {
    record.push(field);
    field = "";
  };
  const pushRecord = () => {
    pushField();
    records.push(record);
    record = [];
  };
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (inQuotes) {
      if (c === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i++;
        } else inQuotes = false;
      } else field += c;
    } else if (c === '"') {
      inQuotes = true;
    } else if (c === ",") {
      pushField();
    } else if (c === "\n") {
      pushRecord();
    } else if (c === "\r") {
      // ignore; \r\n handled by \n
    } else {
      field += c;
    }
  }
  if (field !== "" || record.length) pushRecord();
  return matrixToRows(records);
}

export async function parseTableFile(file: File): Promise<TableRow[]> {
  const name = file.name.toLowerCase();
  if (name.endsWith(".csv") || file.type === "text/csv") {
    return parseCsv(await file.text());
  }
  const mod = await import("read-excel-file/browser");
  const matrix = (await mod.default(file)) as unknown as unknown[][];
  return matrixToRows(matrix);
}
