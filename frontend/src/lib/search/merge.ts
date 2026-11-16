export function mergeSearchHits<T extends { id: string }>(
  previous: T[],
  page: T[],
  append: boolean,
): T[] {
  if (!append) return page;
  const seen = new Set(previous.map((item) => item.id));
  const next = previous.slice();
  for (const item of page) {
    if (seen.has(item.id)) continue;
    seen.add(item.id);
    next.push(item);
  }
  return next;
}
