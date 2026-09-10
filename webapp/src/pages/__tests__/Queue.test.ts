import { beforeEach, expect, it, vi } from "vitest";
import { defineComponent } from "vue";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia } from "pinia";
import { createMemoryHistory, createRouter, RouterView } from "vue-router";
const server = vi.hoisted(() => {
  const instance = { id: 2, name: "Club server", running: false, run_mode: "manual_queue" };
  return { instances: { 2: instance }, instanceList: [instance], selectedInstanceId: 2, load: vi.fn().mockResolvedValue(undefined), selectInstance: vi.fn() };
});
vi.mock("@/stores/server", () => ({ useServerStore: () => server }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ canOperate: true, isAdmin: true }) }));
vi.mock("@/lib/api", () => ({ api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }, ApiError: class extends Error {} }));
import Queue from "../Queue.vue";
import { api } from "@/lib/api";
import { emptyRaceSetup } from "@/lib/useRaceSetupDraft";
const Editor = defineComponent({ name: "RaceSetupEditor", props: ["modelValue"], emits: ["update:modelValue"], template: "<div>Editor</div>" });
beforeEach(() => {
  vi.mocked(api.get).mockImplementation(async (url: string) => ({ items: url === "/api/categories" ? [{ id: 1, name: "Races" }] : [] }) as never);
  vi.mocked(api.put).mockResolvedValue({});
  vi.mocked(api.post).mockReset();
});
it("retries adding a saved setup without creating a duplicate after a queue failure", async () => {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/queue", component: Queue }] });
  await router.push("/queue?instance=2");
  const wrapper = mount(RouterView, { global: { plugins: [createPinia(), router], stubs: {
    Sheet: { props: ["open"], template: '<div v-if="open"><slot /><slot name="footer" /></div>' },
    Modal: { template: "<div />" }, RaceSetupEditor: Editor,
  } } });
  await flushPromises();
  await wrapper.findAll("button").find(button => button.text() === "New race setup")!.trigger("click");
  wrapper.getComponent(Editor).vm.$emit("update:modelValue", { ...emptyRaceSetup(), name: "Sprint", track_key: "spa", class_id: 1, session_id: 2, difficulty_id: 3, time_id: 4 });
  await flushPromises();
  vi.mocked(api.post).mockImplementation(async (url: string) => {
    if (url === "/api/events") return { id: 12 } as never;
    throw new Error("Queue unavailable");
  });
  await wrapper.findAll("button").find(button => button.text().includes("Create & queue"))!.trigger("click"); await flushPromises();
  expect(wrapper.text()).toContain("The setup is saved in your library");
  vi.mocked(api.post).mockResolvedValue({});
  await wrapper.findAll("button").find(button => button.text().includes("Save & queue"))!.trigger("click"); await flushPromises();
  expect(vi.mocked(api.post).mock.calls.filter(([url]) => url === "/api/events")).toHaveLength(1);
  expect(vi.mocked(api.post).mock.calls.filter(([url]) => url === "/api/queue/event/12?instance=2")).toHaveLength(2);
  expect(api.put).toHaveBeenCalledWith("/api/event/12", expect.objectContaining({ name: "Sprint" }));
  expect(wrapper.findComponent(Editor).exists()).toBe(false);
  wrapper.unmount();
});

it("clears completed rows only on the selected server", async () => {
  vi.mocked(api.get).mockImplementation(async (url: string) => ({ items: url.startsWith("/api/queue?") ? [
    { id: 31, instance_id: 2, name: "Finished race", track: "Spa", category: "Club", finished: 1 },
    { id: 32, instance_id: 2, name: "Next race", track: "Spa", category: "Club", finished: 0 },
    { id: 33, instance_id: 7, name: "Another server", track: "Spa", category: "Club", finished: 1 },
  ] : [] }) as never);
  vi.mocked(api.delete).mockResolvedValue({});
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: "/queue", component: Queue }, { path: "/server/:id", component: { template: "Server" } }] });
  await router.push("/queue?instance=2");
  const wrapper = mount(RouterView, { global: { plugins: [createPinia(), router], stubs: { Sheet: true, Modal: true, RaceSetupEditor: true } } });
  await flushPromises();
  await wrapper.findAll("button").find(button => button.text().includes("Clear completed"))!.trigger("click"); await flushPromises();
  expect(api.delete).toHaveBeenCalledExactlyOnceWith("/api/queue/31");
  expect(api.post).not.toHaveBeenCalled();
  wrapper.unmount();
});
