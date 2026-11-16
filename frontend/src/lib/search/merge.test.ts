import { describe, expect, it } from "vitest";

import { mergeSearchHits } from "@/lib/search/merge";

const a = { id: "a", name: "A" };
const b = { id: "b", name: "B" };
const c = { id: "c", name: "C" };

describe("mergeSearchHits", () => {
  it("replaces the list for a new search", () => {
    expect(mergeSearchHits([a], [b], false)).toEqual([b]);
  });

  it("appends the next page and drops ids already shown", () => {
    expect(mergeSearchHits([a, b], [b, c], true)).toEqual([a, b, c]);
  });
});
