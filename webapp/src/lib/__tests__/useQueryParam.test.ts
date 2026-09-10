import { describe, expect, it } from "vitest";
import { defineComponent } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import { createMemoryHistory, createRouter, type Router } from "vue-router";
import { useQueryParam, numberParam, enumParam, type QueryParamOptions } from "../useQueryParam";

// Mount a throwaway component that binds one query param, returning the ref and
// the router so a test can drive and inspect both sides of the sync.
function harness<T extends string | number | null>(key: string, def: T, opts: QueryParamOptions<T>, initialUrl: string) {
  let state!: { value: T };
  const Comp = defineComponent({
    setup() {
      state = useQueryParam(key, def, opts);
      return () => null;
    },
  });
  const router: Router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/x", component: Comp }] });
  return router
    .push(initialUrl)
    .then(() => router.isReady())
    .then(() => {
      mount(Comp, { global: { plugins: [router] } });
      return { get state() { return state; }, router };
    });
}

describe("numberParam", () => {
  const p = numberParam();
  it("parses ids and rejects junk", () => {
    expect(p.parse!("7")).toBe(7);
    expect(p.parse!("")).toBeNull();
    expect(p.parse!("abc")).toBeNull();
  });
  it("serializes, dropping null", () => {
    expect(p.serialize!(7)).toBe("7");
    expect(p.serialize!(null)).toBeNull();
  });
});

describe("enumParam", () => {
  const p = enumParam(["all", "repeating"] as const, "all");
  it("keeps allowed values and falls back otherwise", () => {
    expect(p.parse!("repeating")).toBe("repeating");
    expect(p.parse!("bogus")).toBe("all");
  });
});

describe("useQueryParam", () => {
  it("reads the initial value from the URL", async () => {
    const h = await harness<number | null>("instance", null, numberParam(), "/x?instance=3");
    expect(h.state.value).toBe(3);
  });

  it("falls back to the default when absent", async () => {
    const h = await harness<number | null>("instance", null, numberParam(), "/x");
    expect(h.state.value).toBeNull();
  });

  it("writes changes to the URL with replace (no history growth)", async () => {
    const h = await harness<number | null>("instance", null, numberParam(), "/x");
    const before = window.history.length;
    h.state.value = 5;
    await flushPromises();
    expect(h.router.currentRoute.value.query.instance).toBe("5");
    expect(window.history.length).toBe(before); // replace, not push
  });

  it("drops the param when set back to the default", async () => {
    const h = await harness<number | null>("instance", null, numberParam(), "/x?instance=5");
    h.state.value = null;
    await flushPromises();
    expect(h.router.currentRoute.value.query.instance).toBeUndefined();
  });

  it("preserves unrelated query params", async () => {
    const h = await harness<string>("q", "", {}, "/x?q=alpha&keep=1");
    h.state.value = "beta";
    await flushPromises();
    expect(h.router.currentRoute.value.query.q).toBe("beta");
    expect(h.router.currentRoute.value.query.keep).toBe("1");
  });
});

it("updates mounted filters on same-page navigation and back", async () => {
  const h = await harness<string>("q", "", {}, "/x?q=first&keep=1");
  await h.router.push("/x?q=second&keep=1"); await flushPromises();
  expect(h.state.value).toBe("second");
  h.router.back(); await flushPromises();
  expect(h.state.value).toBe("first");
  expect(h.router.currentRoute.value.query.keep).toBe("1");
});
it("can clear multiple filters through one route update", async () => {
  const h = await harness<string>("q", "", {}, "/x?q=first&group=3&keep=1");
  await h.router.replace({ query: { keep: "1" } }); await flushPromises();
  expect(h.state.value).toBe("");
  expect(h.router.currentRoute.value.query).toEqual({ keep: "1" });
});
