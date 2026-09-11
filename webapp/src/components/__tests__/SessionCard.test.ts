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

it("keeps results, tags, media and attribution available in an expanded drive", async () => {
  const media = { id: "photo-1", kind: "screenshot" as const, url: "/photo.jpg", caption: "At the finish", captured_at: 1 };
  const wrapper = mount(SessionCard, { props: { canOperate: true, defaultOpen: true, guests: [{ id: 7, name: "Alex Guest", created_at: 1 }], session: {
    id: "2", track: { key: "spa", name: "Spa" }, car: { key: "car", name: "Car" }, joined_at: 0, left_at: 1,
    online: false, tags: ["Sunday"], laps_total: 1, best_lap_ms: 60123, segments: [], laps: [{ lap: 1, laptime_ms: 60123, cuts: 0, is_best: true }], drift_runs: [], media: [media],
  } }, global: { stubs: { Modal: { props: ["open"], template: '<div v-if="open"><slot /><slot name="footer" /></div>' } } } });
  try {
    expect(wrapper.get("table").text()).toContain("1:00.123");
    await wrapper.get('input[aria-label="Add session tag"]').setValue("  Club night  ");
    await wrapper.get("form").trigger("submit");
    expect(wrapper.emitted("add-tag")).toEqual([["Club night"]]);
    await wrapper.get('button[aria-label="Remove tag Sunday"]').trigger("click");
    expect(wrapper.emitted("remove-tag")).toEqual([["Sunday"]]);
    await wrapper.get('button[aria-label="Download screenshot"]').trigger("click");
    expect(wrapper.emitted("download-media")).toEqual([[media]]);
    await wrapper.get('button[aria-label="Delete screenshot"]').trigger("click");
    expect(wrapper.emitted("delete-media")).toEqual([[media]]);
    await wrapper.findAll("button").find(b => b.text() === "Reassign driver")!.trigger("click");
    await wrapper.findAll("button").find(b => b.text() === "Alex Guest")!.trigger("click");
    expect(wrapper.emitted("assign")).toEqual([[7]]);
    await wrapper.findAll("button").find(b => b.text() === "Delete session")!.trigger("click");
    expect(wrapper.emitted("delete-session")).toHaveLength(1);
  } finally { wrapper.unmount(); }
});
