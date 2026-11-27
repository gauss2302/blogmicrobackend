import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** Locale date and time for an ISO timestamp; the raw string if it does not parse. */
export function formatDateTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  // toLocaleString, not toLocaleDateString: the latter throws on timeStyle.
  return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

/** UTF-8 length: the backend limits names, bio and passwords in bytes, not characters. */
export function utf8Bytes(value: string): number {
  return new TextEncoder().encode(value).length;
}
