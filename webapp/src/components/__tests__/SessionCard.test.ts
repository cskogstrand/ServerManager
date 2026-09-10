import { expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import SessionCard from "../SessionCard.vue";
vi.mock("@/stores/content", () => ({ useContentStore: () => ({ trackByKey: () => null }) }));
it("exposes a focusable session toggle that opens and closes the details", async () => {
  const wrapper = mount(SessionCard, { props: { canOperate: false, session: {
    id: "1", track: { key: "spa", name: "Spa" }, car: { key: "car", name: "Car" }, joined_at: 0, left_at: 1,
    online: false, tags: [], laps_total: 0, segments: [], laps: [], drift_runs: [], media: [],
  } }, global: { stubs: { Modal: true } } });
  const toggle = wrapper.get('button[aria-label="Expand session at Spa"]');
  expect((toggle.element as HTMLButtonElement).tabIndex).toBe(0);
  expect(toggle.attributes("aria-expanded")).toBe("false");
  await toggle.trigger("click");
  expect(toggle.attributes("aria-expanded")).toBe("true");
  await toggle.trigger("click");
  expect(toggle.attributes("aria-expanded")).toBe("false");
  wrapper.unmount();
});
