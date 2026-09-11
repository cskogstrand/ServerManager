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

it("keeps the four main destinations labelled and selects Sessions for server operations", async () => {
 const router=await open('/sessions/4');
 expect(wrapper.findAll('.pitlane-nav a').map(a=>a.text())).toEqual(['Today','Sessions','Live','Drivers']);
 expect(wrapper.get('.pitlane-nav a[href="/sessions"]').attributes('aria-current')).toBe('page');
 await router.push('/server/2');await flushPromises();
 expect(wrapper.get('.pitlane-nav a[href="/sessions"]').attributes('aria-current')).toBe('page');
 expect(server.selectInstance).toHaveBeenCalledWith(2);
 await router.push('/guest-drivers/3');await flushPromises();
 expect(wrapper.get('.pitlane-nav a[href="/drivers"]').attributes('aria-current')).toBe('page');
 expect(wrapper.get('[aria-label="Mobile navigation"]').findAll('a')).toHaveLength(5);
});
it("retains viewer navigation and changes appearance even when storage is unavailable", async () => {
 auth.isAdmin=false;auth.canOperate=false;await open('/');
 expect(wrapper.findAll('.pitlane-nav a')).toHaveLength(4);
 await wrapper.get('[aria-label="Account and preferences"]').trigger('click');
 expect(wrapper.text()).toContain('Preferences');
 const themeButton=(name:string)=>wrapper.findAll('button').find(b=>b.text()===name)!;
 await themeButton('Light').trigger('click');
 expect(document.documentElement.dataset.theme).toBe('light');
 expect(localStorage.setItem).toHaveBeenCalledWith('theme','light');
 vi.mocked(localStorage.setItem).mockImplementation(()=>{throw new Error('Storage unavailable')});
 await themeButton('Dark').trigger('click');
 expect(document.documentElement.dataset.theme).toBe('dark');
});
