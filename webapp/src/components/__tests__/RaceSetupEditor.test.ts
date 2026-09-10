import { expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { emptyRaceSetup } from "@/lib/useRaceSetupDraft";

const confirm = vi.hoisted(() => ({ ask: vi.fn() }));
vi.mock("@/stores/confirm", () => ({ useConfirmStore: () => confirm }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ isAdmin: true, canOperate: true }) }));
vi.mock("@/stores/server", () => ({ useServerStore: () => ({ instances: {} }) }));
vi.mock("@/stores/content", () => ({ useContentStore: () => ({ load: vi.fn(), trackByKey: () => undefined }) }));
vi.mock("@/lib/api", () => ({ ApiError: class extends Error {}, api: { get: vi.fn(async (url: string) => {
  if (url === "/api/categories") return { items: [{ id: 9, name: "Templates" }] };
  if (url === "/api/category/9") return { events: [{ Id: 12, name: "GT3 sprint", CacheTrackKey: "monza", TrackName: "Monza", class: 1, session: 2, time: 3, difficulty: 4 }] };
  return { items: [] };
}) } }));
import RaceSetupEditor from "../RaceSetupEditor.vue";

it("favorite cards respect cancellation and preserve the edited race identity when applying a template", async () => {
  const draft = { ...emptyRaceSetup(), id: 28, name: "Club championship", track_key: "spa" };
  const wrapper = mount(RaceSetupEditor, { props: { modelValue: draft }, global: { stubs: { TrackImage: true, TrackPicker: true, InlinePresetSheet: true } } });
  try {
    await flushPromises();
    const favorite = wrapper.get('[aria-label="Use template GT3 sprint"]');
    confirm.ask.mockResolvedValueOnce(false);
    await favorite.trigger("click"); await flushPromises();
    expect(wrapper.emitted("update:modelValue")).toBeUndefined();
    expect(draft.track_key).toBe("spa");
    confirm.ask.mockResolvedValueOnce(true);
    await favorite.trigger("click"); await flushPromises();
    expect(wrapper.emitted("update:modelValue")?.[0][0]).toMatchObject({ id: 28, name: "Club championship", track_key: "monza", class_id: 1, session_id: 2, time_id: 3, difficulty_id: 4 });
  } finally { wrapper.unmount(); }
});
