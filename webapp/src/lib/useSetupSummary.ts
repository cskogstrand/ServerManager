import { ref } from "vue";
import { api, ApiError } from "@/lib/api";
import type { DropDownList } from "@/types/generated";

export interface SetupBlocker {
  step: string;
  message: string;
}

export interface SetupSummary {
  install_path: string;
  acserver_found: boolean;
  cfg_filled: boolean;
  mod_filled: boolean;
  content: { tracks: number; cars: number; weathers: number };
  presets: { difficulties: number; sessions: number; classes: number; times: number };
  events: number;
  instances: { id: number; name: string; run_mode: string; queue_pending: number; is_running: boolean }[];
  port_conflict: string;
  config: {
    name: string;
    has_password: boolean;
    has_admin_password: boolean;
    engine: string;
    max_clients: number;
    register_to_lobby: boolean;
  };
  preset_lists: {
    difficulties: DropDownList[];
    sessions: DropDownList[];
    classes: DropDownList[];
    times: DropDownList[];
  };
  groups: DropDownList[];
  suggested_ports: {
    udp_port: number;
    tcp_port: number;
    http_port: number;
    plugin_port: number;
    plugin_listen_port: number;
  };
  blocking: SetupBlocker[];
  can_start: boolean;
}

// useSetupSummary fetches and exposes the aggregated setup state that drives the
// workbench. reload() re-pulls after any inline fix so step status self-updates.
export function useSetupSummary() {
  const summary = ref<SetupSummary | null>(null);
  const loading = ref(true);
  const error = ref("");

  async function reload() {
    try {
      summary.value = await api.get<SetupSummary>("/api/setup/summary");
      error.value = "";
    } catch (e) {
      error.value = e instanceof ApiError ? e.message : String(e);
    } finally {
      loading.value = false;
    }
  }

  return { summary, loading, error, reload };
}

// Setup step ids and the order they appear in the workbench rail.
export const SETUP_STEPS = ["install", "content", "server", "instance", "race", "run"] as const;
export type SetupStep = (typeof SETUP_STEPS)[number];

// stepDone reports whether a given step's gate is satisfied by the summary.
export function stepDone(s: SetupSummary | null, step: SetupStep): boolean {
  if (!s) return false;
  switch (step) {
    case "install":
      return s.acserver_found;
    case "content":
      return s.content.tracks > 0 && s.content.cars > 0;
    case "server":
      return s.cfg_filled;
    case "instance":
      return s.instances.length > 0 && s.port_conflict === "";
    case "race":
      return s.events > 0 && presetsReady(s);
    case "run":
      return s.instances.some((i) => i.queue_pending > 0 || i.run_mode === "repeat_event");
  }
}

export function presetsReady(s: SetupSummary): boolean {
  const p = s.presets;
  return p.classes > 0 && p.sessions > 0 && p.times > 0 && p.difficulties > 0;
}

// firstIncompleteStep is where a returning user resumes.
export function firstIncompleteStep(s: SetupSummary | null): SetupStep {
  for (const step of SETUP_STEPS) {
    if (!stepDone(s, step)) return step;
  }
  return "run";
}
