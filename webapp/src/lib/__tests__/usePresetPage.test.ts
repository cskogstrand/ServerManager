import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defineComponent } from "vue";
import { mount, flushPromises, type VueWrapper } from "@vue/test-utils";
import { createMemoryHistory, createRouter, RouterView } from "vue-router";
import { createPinia } from "pinia";
import { useConfirmStore } from "@/stores/confirm";
import { usePresetPage } from "../usePresetPage";

vi.mock("@/lib/api", () => ({ api: { get: vi.fn().mockResolvedValue({}) }, ApiError: class extends Error {} }));
vi.mock("@/lib/restartServersForTemplate", () => ({ restartServersUsingTemplate: vi.fn().mockResolvedValue(0) }));
const resource = {
  plural: "classes", singular: "class",
  list: vi.fn().mockResolvedValue([{ id: 1, name: "First" }, { id: 2, name: "Second" }]),
  get: vi.fn(async (id: number) => ({ id, name: `Preset ${id}` })),
  create: vi.fn().mockResolvedValue(3), update: vi.fn().mockResolvedValue({}), remove: vi.fn(),
};
let wrapper: VueWrapper;
let page: ReturnType<typeof usePresetPage<{ id: number; name: string }>>;
const Page = defineComponent({ setup() { page = usePresetPage(resource); return () => null; } });
async function setup() {
  const pinia = createPinia();
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/presets", component: Page }, { path: "/done", component: { template: "Done" } }] });
  await router.push("/presets?sel=1");
  wrapper = mount(RouterView, { global: { plugins: [pinia, router] } });
  await flushPromises();
  return { router, confirm: useConfirmStore(pinia) };
}
beforeEach(() => vi.clearAllMocks());
afterEach(() => wrapper?.unmount());

it("keeps an edited preset when switching or creating is cancelled", async () => {
  const { confirm } = await setup();
  page.form.value!.name = "Unsaved";
  await page.select(1);
  expect(confirm.open).toBe(false);
  const switching = page.select(2);
  expect(confirm.open).toBe(true);
  confirm.answer(false); await switching;
  expect(page.selectedId.value).toBe(1);
  expect(page.form.value!.name).toBe("Unsaved");
  const creating = page.create("Third"); confirm.answer(false); await creating;
  expect(resource.create).not.toHaveBeenCalled();
  const duplicating = page.duplicate(2); confirm.answer(false); await duplicating;
  expect(resource.create).not.toHaveBeenCalled();
});

it("loads the chosen preset after discarding and guards browser navigation", async () => {
  const { router, confirm } = await setup();
  page.form.value!.name = "Unsaved";
  const switching = page.select(2); confirm.answer(true); await switching; await flushPromises();
  expect(router.currentRoute.value.query.sel).toBe("2");
  expect(page.form.value!.name).toBe("Preset 2");
  page.form.value!.name = "Keep me";
  const navigating = router.push("/presets?sel=1"); await flushPromises();
  expect(confirm.open).toBe(true);
  confirm.answer(false); await navigating;
  expect(router.currentRoute.value.query.sel).toBe("2");
  expect(page.form.value!.name).toBe("Keep me");
  const leaving = router.push("/done"); await flushPromises();
  confirm.answer(false); await leaving;
  expect(router.currentRoute.value.path).toBe("/presets");
});

it("loads same-page URL selections and preserves the original form when loading fails", async () => {
  const { router } = await setup();
  await router.push("/presets?sel=2"); await flushPromises();
  expect(page.selectedId.value).toBe(2);
  expect(page.form.value!.name).toBe("Preset 2");
  resource.get.mockRejectedValueOnce(new Error("Offline"));
  await router.push("/presets?sel=1"); await flushPromises();
  expect(page.error.value).toContain("Offline");
  expect(page.selectedId.value).toBe(2);
  expect(router.currentRoute.value.query.sel).toBe("2");
  await page.save();
  expect(resource.update).toHaveBeenCalledWith(2, { id: 2, name: "Preset 2" });
});
