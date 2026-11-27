/**
 * Where to go after login. `raw` is the already-decoded `?next=` value; only a
 * target that resolves to `origin` is allowed, and only its path and query are
 * kept. Prefix checks are not enough: the URL parser treats `\` as `/` and
 * strips tabs, so `/\evil.example` and `/\t/evil.example` mean `//evil.example`.
 */
export function safeNextPath(raw: string | null, origin: string): string {
  if (!raw) return "/app";
  try {
    const url = new URL(raw, origin);
    return url.origin === origin ? `${url.pathname}${url.search}` : "/app";
  } catch {
    return "/app";
  }
}
