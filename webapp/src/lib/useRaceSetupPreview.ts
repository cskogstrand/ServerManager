import { api } from "@/lib/api";
import type { RaceSetupDraft } from "@/lib/useRaceSetupDraft";

// Result of POST /api/server/render-preview — the rendered config plus the
// facts a review section needs to flag problems before save/start.
export interface PreviewResult {
  server_cfg: string;
  entry_list: string;
  grid_count: number;
  requested_grid: number;
  pitboxes: number;
  max_clients: number;
  warnings: string[];
  render_errors: string[];
}

// previewRaceSetup renders a draft (or a saved event, when it has an id)
// against an instance without persisting anything.
export function previewRaceSetup(draft: RaceSetupDraft, instanceId?: number | null): Promise<PreviewResult> {
  const body: Record<string, unknown> = { instance_id: instanceId ?? 0 };
  if (draft.id) {
    body.event_id = draft.id;
  } else {
    body.name = draft.name?.trim() ?? "";
    body.track_key = draft.track_key;
    body.track_config = draft.track_config;
    body.class_id = draft.class_id;
    body.session_id = draft.session_id;
    body.time_id = draft.time_id;
    body.difficulty_id = draft.difficulty_id;
    body.race_laps = draft.race_laps ?? 0;
    body.strategy = draft.strategy ?? 1;
  }
  return api.post<PreviewResult>("/api/server/render-preview", body);
}
