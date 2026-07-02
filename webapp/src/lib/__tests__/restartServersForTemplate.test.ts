import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { createPinia, setActivePinia } from "pinia";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), post: vi.fn() },
  ApiError: class ApiError extends Error {},
}));

import { api } from "@/lib/api";
import { restartServersUsingTemplate } from "@/lib/restartServersForTemplate";
import { useConfirmStore } from "@/stores/confirm";

const apiGet = api.get as Mock;
const apiPost = api.post as Mock;

function instance(id: number, name: string, running: boolean) {
  return {
    id,
    name,
    udp_port: 9600,
    tcp_port: 9600,
    http_port: 8081,
    plugin_port: 5000,
    plugin_listen_port: 5001,
    is_running: running,
    players: running ? 1 : 0,
    run_mode: "manual_queue",
    repeat_event_id: null,
    repeat_event: null,
    scheduled_start: null,
    start_on_boot: 0,
    drift_score_enabled: 0,
    drift_scoring_mode_id: 1,
    allow_wrong_way: 0,
    stream_enabled: 0,
    stream_embed_url: null,
    stream_status_url: null,
    spectator_enabled: 0,
    spectator_driver_name: null,
    spectator_guid: null,
    spectator_car_key: null,
    spectator_skin_key: null,
  };
}

beforeEach(() => {
  setActivePinia(createPinia());
  apiGet.mockReset();
  apiPost.mockReset();
});

describe("restartServersUsingTemplate", () => {
  it("asks once and restarts running servers using the saved template", async () => {
    const confirm = useConfirmStore();
    vi.spyOn(confirm, "ask").mockResolvedValue(true);
    apiPost.mockResolvedValue({});
    apiGet.mockImplementation((url: string) => {
      if (url === "/api/instances") {
        return Promise.resolve({
          instances: [instance(1, "One", true), instance(2, "Stopped", false), instance(3, "Other", true)],
        });
      }
      if (url === "/api/server/status?instance=1") return Promise.resolve({ current_event: { class_id: 7 } });
      if (url === "/api/server/status?instance=3") return Promise.resolve({ current_event: { class_id: 8 } });
      throw new Error(`unexpected ${url}`);
    });

    const restarted = await restartServersUsingTemplate("classes", 7);

    expect(restarted).toBe(1);
    expect(confirm.ask).toHaveBeenCalledWith(expect.objectContaining({ message: "One is using this template. Restart it now?" }));
    expect(apiGet).not.toHaveBeenCalledWith("/api/server/status?instance=2");
    expect(apiPost).toHaveBeenCalledWith("/api/server/restart?instance=1");
    expect(apiPost).not.toHaveBeenCalledWith("/api/server/restart?instance=3");
  });
});
