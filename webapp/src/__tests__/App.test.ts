import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { mount, flushPromises, type VueWrapper } from "@vue/test-utils";
import { createMemoryHistory, createRouter } from "vue-router";

const auth = vi.hoisted(() => ({ loggedIn: true, isAdmin: true, canOperate: true, user: { name: "Admin" }, logout: vi.fn() }));
const server = vi.hoisted(() => ({
  instances: { 2: { id: 2, name: "Club server" } }, instanceList: [{ id: 2, name: "Club server", running: false }],
  selectedInstanceId: 2, loaded: true, connected: true, statusErrors: {}, loadError: "", lastUpdatedAt: null,
  bootstrapLiveState: vi.fn().mockResolvedValue(undefined), connect: vi.fn(), disconnect: vi.fn(), recoverRunning: vi.fn(), selectInstance: vi.fn(),
}));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => auth }));
vi.mock("@/stores/server", () => ({ useServerStore: () => server }));
vi.mock("@/lib/api", () => ({ api: { get: vi.fn().mockResolvedValue({ cfg_filled: 1, mod_filled: 1 }) } }));
import App from "../App.vue";

let wrapper: VueWrapper;
beforeEach(() => { auth.isAdmin = true; auth.canOperate = true; document.documentElement.dataset.theme = "dark"; vi.stubGlobal("localStorage", { setItem: vi.fn() }); });
afterEach(() => { wrapper?.unmount(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });
async function open(path: string) {
  const page = { template: "<h1>Workspace page</h1>" };
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: "/", component: page },
    { path: "/presets/:kind", component: page, meta: { admin: true } },
    { path: "/server/:id", name: "server-detail", component: page },
    { path: "/:pathMatch(.*)*", component: page },
  ] });
  await router.push(path);
  wrapper = mount(App, { global: { plugins: [router], stubs: {
    Toaster: true, ConfirmDialog: true,
    Sheet: { props: ["open"], template: '<div v-if="open"><slot /></div>' },
  } } });
  await flushPromises();
  return router;
}

it("keeps navigation labelled and selects the right section for nested routes and server context", async () => {
  const router = await open("/presets/sessions");
  expect(wrapper.get('.rail-nav [aria-label="Templates"]').attributes("aria-current")).toBe("page");
  expect(wrapper.get('[aria-label="Admin"]').attributes("aria-current")).toBeUndefined();
  await wrapper.get('[aria-label="Expand navigation"]').trigger("click");
  expect(wrapper.get('[aria-label="Collapse navigation"]').attributes("aria-expanded")).toBe("true");
  await router.push("/server/2");
  expect(wrapper.get('.rail-nav [aria-label="Race Control"]').attributes("aria-current")).toBe("page");
  expect(wrapper.get('.rail-nav [aria-label="Dashboard"]').attributes("aria-current")).toBeUndefined();
  expect(wrapper.get('.rail-nav [aria-label="Run Plan"]').attributes("href")).toBe("/queue?instance=2");
});

it("retains viewer navigation and synchronizes header and account appearance controls, even without storage", async () => {
  auth.isAdmin = false; auth.canOperate = false;
  await open("/");
  expect(wrapper.find('.rail-nav [aria-label="Templates"]').exists()).toBe(false);
  expect(wrapper.find('[aria-label="Admin"]').exists()).toBe(false);
  await wrapper.get('[aria-label="More menu"]').trigger("click");
  expect(wrapper.get('[aria-label="All navigation"]').text()).toContain("Preferences");
  await wrapper.findAll('.workspace-header .theme-switch button')[0].trigger("click");
  expect(document.documentElement.dataset.theme).toBe("light");
  expect(localStorage.setItem).toHaveBeenCalledWith("theme", "light");
  expect(wrapper.findAll('.theme-switch button[aria-pressed="true"]').every(button => button.text() === "Light")).toBe(true);
  vi.mocked(localStorage.setItem).mockImplementation(() => { throw new Error("Storage unavailable"); });
  await wrapper.findAll('.workspace-header .theme-switch button')[1].trigger("click");
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(wrapper.findAll('.theme-switch button[aria-pressed="true"]').every(button => button.text() === "Dark")).toBe(true);
});
