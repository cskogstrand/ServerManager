import { expect, it, vi } from "vitest";
vi.mock("@/lib/api", () => ({ api: { post: vi.fn().mockResolvedValue({}) } }));
import { api } from "@/lib/api";
import { previewRaceSetup } from "../useRaceSetupPreview";
import { emptyRaceSetup } from "../useRaceSetupDraft";
it("previews current edits rather than silently rendering the saved event", async () => {
  const draft = { ...emptyRaceSetup(), id: 9, name: "  Edited race  ", track_key: "spa", track_config: "gp", class_id: 2, session_id: 3, time_id: 4, difficulty_id: 5, race_laps: 12 };
  await previewRaceSetup(draft, 7);
  expect(api.post).toHaveBeenCalledWith("/api/server/render-preview", {
    instance_id: 7, name: "Edited race", track_key: "spa", track_config: "gp", class_id: 2, session_id: 3, time_id: 4, difficulty_id: 5, race_laps: 12, strategy: 1,
  });
});
