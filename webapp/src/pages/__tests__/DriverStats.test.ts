import { expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createMemoryHistory, createRouter } from "vue-router";
import { listDrivers } from "@/lib/driversApi";
import DriverStats from "../DriverStats.vue";

vi.mock("@/lib/driversApi", async importOriginal => ({
  ...await importOriginal<typeof import("@/lib/driversApi")>(), listDrivers: vi.fn(),
}));
vi.mock("@/stores/server", () => ({ useServerStore: () => ({ instanceList: [] }) }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ canOperate: false }) }));

it("keeps account and guest profile links, sorting and query filters in the unified directory", async () => {
  const base = { online: false, first_seen: 1, last_seen: 1, sessions: 2, total_laps: 3, best_lap_ms: 0, podiums: 0, last_result: null, drift_trend: [] };
  vi.mocked(listDrivers).mockResolvedValue([
    { ...base, guid: "account-1", name: "Zoe Account", best_drift: 20 },
    { ...base, guid: "", name: "Alex Guest", best_drift: 10, is_guest: true, guest_id: 7 },
  ]);
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: "/drivers", component: DriverStats },
    { path: "/drivers/:guid", name: "driver-detail", component: { template: "Profile" } },
    { path: "/guest-drivers/:id", name: "guest-driver-detail", component: { template: "Guest" } },
    { path: "/leaderboard", component: { template: "Records" } },
  ] });
  await router.push("/drivers");
  const wrapper = mount(DriverStats, { global: { plugins: [router] } });
  try {
    await flushPromises();
    expect(wrapper.findAll(".pitlane-driver-row").map(row => row.attributes("href"))).toEqual(["/drivers/account-1", "/guest-drivers/7"]);
    expect(wrapper.text()).not.toContain("Manage guests");
    await wrapper.findAll("button").find(b => b.text() === "Name")!.trigger("click");
    await flushPromises();
    expect(router.currentRoute.value.query.sort).toBe("name");
    expect(wrapper.find(".pitlane-driver-row").attributes("href")).toBe("/guest-drivers/7");
    await wrapper.get('input[type="search"]').setValue("Zoe");
    await flushPromises();
    expect(router.currentRoute.value.query.q).toBe("Zoe");
    expect(wrapper.findAll(".pitlane-driver-row")).toHaveLength(1);
    await wrapper.findAll("button").find(b => b.text() === "On track only")!.trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("No drivers match.");
    await wrapper.findAll("button").find(b => b.text() === "Clear filters")!.trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".pitlane-driver-row")).toHaveLength(2);
  } finally { wrapper.unmount(); }
});
