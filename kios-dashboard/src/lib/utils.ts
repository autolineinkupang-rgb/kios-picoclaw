import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/** Merge Tailwind class names, resolving conflicts (last wins). */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/**
 * Case-insensitive search across one or more fields. `q` must already be
 * lowercased (callers do `query.trim().toLowerCase()`). Each field is coerced
 * with `?? ""` so a null/undefined value from unexpected data never throws.
 */
export function matchesQuery(q: string, ...fields: (string | null | undefined)[]): boolean {
  return fields.some((f) => (f ?? "").toLowerCase().includes(q));
}
