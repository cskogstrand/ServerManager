import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { defineComponent, reactive } from "vue";
import { mount, flushPromises, type VueWrapper } from "@vue/test-utils";
import { createMemoryHistory, createRouter, RouterView } from "vue-router";
import { createPinia } from "pinia";
vi.mock("@/lib/api", () => ({ api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }, ApiError: class extends Error {}, csrfToken: () => "" }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ canOperate: true, isAdmin: true }) }));
vi.mock("@/stores/server", () => ({ useServerStore: () => server }));
vi.mock("@/stores/content", () => ({ useContentStore: () => ({ load: vi.fn().mockResolvedValue(undefined), trackByKey: () => null, carByKey: () => null, cars: [] }) }));
import { api } from "@/lib/api";
import Events from "../Events.vue";
import StreamingCapture from "../StreamingCapture.vue";
import InstallationSettings from "../InstallationSettings.vue";
import SetupWorkbench from "../SetupWorkbench.vue";
import { emptyRaceSetup } from "@/lib/useRaceSetupDraft";
const instance = reactive({ id: 2, name: "Club", running: false, run_mode: "manual_queue" });
const server = { instances: { 2: instance }, instanceList: [instance], selectedInstanceId: 2, selectInstance: vi.fn(), load: vi.fn().mockResolvedValue(undefined) };
const Editor = defineComponent({ name: "RaceSetupEditor", props: ["modelValue"], emits: ["update:modelValue"], template: "<div>Editor</div>" });
const summary = { install_path: "/corsa", acserver_found: true, cfg_filled: true, mod_filled: true, content: { tracks: 1, cars: 1, weathers: 1 }, presets: { difficulties: 1, sessions: 1, times: 1, classes: 1 }, events: 1, instances: [{ id: 2, name: "Club", queue_pending: 1, run_mode: "manual_queue", is_running: false }], groups: [{ id: 1, name: "Races" }], port_conflict: "", config: {}, preset_lists: {}, suggested_ports: {}, blocking: [], can_start: true };
let wrapper: VueWrapper;
async function setup(url: string) {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: "/events", component: Events }, { path: "/streaming", component: StreamingCapture },
    { path: "/installation", component: InstallationSettings }, { path: "/setup", component: SetupWorkbench },
    { path: "/:pathMatch(.*)*", component: { template: "Other page" } },
    { path: "/queue", component: { template: "Run plan" } }, { path: "/server/:id", component: { template: "Race Control" } },
  ] });
  await router.push(url);
  wrapper = mount(RouterView, { global: { plugins: [createPinia(), router], stubs: {
    Modal: { props: ["open"], template: '<div v-if="open" role="dialog"><slot /><slot name="footer" /></div>' },
    Sheet: true, RaceSetupEditor: Editor, TrackImage: true, AdminBackButton: true,
  } } });
  await flushPromises();
  return router;
}
const button = (text: string) => wrapper.findAll("button").find(b => b.text() === text)!;
beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("localStorage", { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() }); instance.running = false; instance.run_mode = "manual_queue";
  vi.mocked(api.get).mockImplementation(async (url: string) => {
    if (url === "/api/categories") return { items: [{ id: 1, name: "Races" }] } as never;
    if (url === "/api/category/1") return { events: [{ Id: 10, name: "Sprint", CacheTrackKey: "spa" }] } as never;
    if (url === "/api/config") return { install_path: "/corsa", capture_trigger_score: 1000 } as never;
    if (url === "/api/driver-streams") return { streams: [] } as never;
    if (url === "/api/setup/summary") return summary as never;
    throw new Error(`Unexpected GET ${url}`);
  });
  vi.mocked(api.post).mockResolvedValue({}); vi.mocked(api.put).mockResolvedValue({});
});
afterEach(() => { wrapper?.unmount(); vi.unstubAllGlobals(); });

it("explains queue ordering and retries a start without adding a second queue entry", async () => {
  await setup("/events");
  await button("Queue & start server").trigger("click"); await flushPromises();
  const dialog = wrapper.get('[role="dialog"]');
  expect(dialog.text()).toContain("Races already ahead of it run first");
  vi.mocked(api.post).mockImplementation(async (url: string) => { if (url.includes("/start")) throw new Error("Start failed"); return {}; });
  await dialog.findAll("button").find(b => b.text() === "Queue & start server")!.trigger("click"); await flushPromises();
  expect(dialog.text()).toContain("already queued");
  vi.mocked(api.post).mockResolvedValue({});
  await button("Retry start").trigger("click"); await flushPromises();
  expect(vi.mocked(api.post).mock.calls.filter(([url]) => url === "/api/queue/event/10?instance=2")).toHaveLength(1);
  expect(vi.mocked(api.post).mock.calls.filter(([url]) => url === "/api/server/start?instance=2")).toHaveLength(2);
});

it("explains why a repeating server cannot accept a queued race", async () => {
  instance.run_mode = "repeat_event";
  await setup("/events");
  await button("Add to run plan").trigger("click"); await flushPromises();
  const dialog = wrapper.get('[role="dialog"]');
  expect(dialog.text()).toContain("Switch to the manual queue");
  expect(dialog.findAll("button").find(b => b.text() === "Add to run plan")!.attributes("disabled")).toBeDefined();
  expect(dialog.get("a").attributes("href")).toBe("/queue?instance=2");
  expect(api.post).not.toHaveBeenCalled();
});

it("preserves unsaved capture settings after saving a driver stream", async () => {
  await setup("/streaming");
  await wrapper.get("#captrigger").setValue("2500");
  await button("Add driver stream").trigger("click");
  await wrapper.get("#dsguid").setValue("driver-1");
  await button("Create").trigger("click"); await flushPromises();
  expect((wrapper.get("#captrigger").element as HTMLInputElement).value).toBe("2500");
  expect(vi.mocked(api.get).mock.calls.filter(([url]) => url === "/api/config")).toHaveLength(1);
  await button("Save capture settings").trigger("click"); await flushPromises();
  expect(api.put).toHaveBeenCalledWith("/api/config", expect.objectContaining({ capture_trigger_score: 2500 }));
});

it("shows recoverable installation errors and only confirms a completed path check", async () => {
  vi.mocked(api.get).mockRejectedValueOnce(new Error("Settings unavailable"));
  await setup("/installation");
  expect(wrapper.text()).toContain("Settings unavailable");
  await button("Retry loading settings").trigger("click"); await flushPromises();
  const fetch = vi.fn().mockRejectedValueOnce(new Error("Connection lost")); vi.stubGlobal("fetch", fetch);
  await button("Check path").trigger("click"); await flushPromises();
  expect(wrapper.get("#installpath").attributes("aria-invalid")).toBe("true");
  expect(wrapper.text()).toContain("Connection lost");
  fetch.mockResolvedValueOnce({ ok: true, json: async () => ({ result: true }) });
  await button("Check path").trigger("click"); await flushPromises();
  expect(wrapper.text()).toContain("Server binary found");
  await wrapper.get("#installpath").setValue("/new-path");
  expect(wrapper.text()).not.toContain("Server binary found");
});

it("retries the setup workbench start without requeuing the saved race", async () => {
  const router = await setup("/setup?step=race");
  wrapper.getComponent(Editor).vm.$emit("update:modelValue", { ...emptyRaceSetup(), name: "Sprint", track_key: "spa", class_id: 1, session_id: 2, time_id: 3, difficulty_id: 4 });
  await flushPromises();
  vi.mocked(api.post).mockImplementation(async (url: string) => {
    if (url === "/api/events") return { id: 10 } as never;
    if (url.includes("/start")) throw new Error("Start failed");
    return {};
  });
  await button("Save race setup").trigger("click"); await flushPromises();
  await button("Start server").trigger("click"); await flushPromises();
  expect(wrapper.text()).toContain("already queued");
  vi.mocked(api.post).mockResolvedValue({});
  await button("Retry start").trigger("click"); await flushPromises();
  expect(vi.mocked(api.post).mock.calls.filter(([url]) => url === "/api/queue/event/10?instance=2")).toHaveLength(1);
  expect(router.currentRoute.value.path).toBe("/server/2");
});
