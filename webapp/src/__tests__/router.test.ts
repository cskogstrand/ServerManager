import { beforeEach, expect, it, vi } from "vitest";
const auth = vi.hoisted(() => ({ loggedIn: true, isAdmin: false, canOperate: false, ensureChecked: vi.fn().mockResolvedValue(undefined) }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => auth }));
import router from "../router";
beforeEach(() => { auth.loggedIn = true; auth.isAdmin = false; auth.canOperate = false; });
it("explains administrator access instead of silently returning to the overview", async () => {
  await router.push("/setup");
  expect(router.currentRoute.value.name).toBe("access-denied");
  expect(router.currentRoute.value.query.role).toBe("admin");
});
it("explains operator-only access for viewers", async () => {
  await router.push("/presets");
  expect(router.currentRoute.value.name).toBe("access-denied");
  expect(router.currentRoute.value.query.role).toBe("operator");
});
it("allows stewards to reach their race tools", async () => {
  auth.canOperate = true;
  await router.push("/presets"); expect(router.currentRoute.value.name).toBe("preset-templates");
});
it("preserves the destination when login is required", async () => {
  auth.loggedIn = false;
  await router.push("/queue?instance=2");
  expect(router.currentRoute.value.name).toBe("login");
  expect(router.currentRoute.value.query.redirect).toBe("/queue?instance=2");
});
it("keeps global broadcast in the workspace and dedicated displays standalone", () => {
  expect(router.resolve("/broadcast").meta.bare).toBeUndefined();
  expect(router.resolve("/server/2/broadcast").meta.bare).toBe(true);
  expect(router.resolve("/leaderboard").meta.bare).toBe(true);
});
