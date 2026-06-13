import { describe, expect, it } from "vitest";
import { stepDone, firstIncompleteStep, presetsReady, type SetupSummary } from "@/lib/useSetupSummary";

function makeSummary(over: Partial<SetupSummary> = {}): SetupSummary {
  return {
    install_path: "/ac",
    acserver_found: true,
    cfg_filled: true,
    mod_filled: true,
    content: { tracks: 5, cars: 10, weathers: 3 },
    presets: { difficulties: 1, sessions: 1, classes: 1, times: 1 },
    events: 2,
    instances: [{ id: 1, name: "S1", run_mode: "manual_queue", queue_pending: 1, is_running: false }],
    port_conflict: "",
    config: { name: "S", has_password: false, has_admin_password: true, engine: "kunos", max_clients: 24, register_to_lobby: true },
    preset_lists: { difficulties: [], sessions: [], classes: [], times: [] },
    groups: [],
    suggested_ports: { udp_port: 9601, tcp_port: 9601, http_port: 8082, plugin_port: 5002, plugin_listen_port: 5003 },
    blocking: [],
    can_start: true,
    ...over,
  };
}

describe("useSetupSummary step logic", () => {
  it("a complete summary marks every step done", () => {
    const s = makeSummary();
    for (const step of ["install", "content", "server", "instance", "race", "run"] as const) {
      expect(stepDone(s, step)).toBe(true);
    }
    expect(firstIncompleteStep(s)).toBe("run");
  });

  it("fresh install resumes at the install step", () => {
    const s = makeSummary({
      acserver_found: false,
      content: { tracks: 0, cars: 0, weathers: 0 },
      cfg_filled: false,
      presets: { difficulties: 0, sessions: 0, classes: 0, times: 0 },
      events: 0,
      instances: [],
    });
    expect(stepDone(s, "install")).toBe(false);
    expect(firstIncompleteStep(s)).toBe("install");
  });

  it("valid path but empty cache resumes at content", () => {
    const s = makeSummary({ content: { tracks: 0, cars: 0, weathers: 0 }, events: 0 });
    expect(stepDone(s, "install")).toBe(true);
    expect(stepDone(s, "content")).toBe(false);
    expect(firstIncompleteStep(s)).toBe("content");
  });

  it("content cached but no event resumes at race", () => {
    const s = makeSummary({ events: 0, instances: [{ id: 1, name: "S1", run_mode: "manual_queue", queue_pending: 0, is_running: false }] });
    expect(stepDone(s, "content")).toBe(true);
    expect(stepDone(s, "instance")).toBe(true);
    expect(stepDone(s, "race")).toBe(false);
    expect(firstIncompleteStep(s)).toBe("race");
  });

  it("everything set but nothing queued resumes at run", () => {
    const s = makeSummary({ instances: [{ id: 1, name: "S1", run_mode: "manual_queue", queue_pending: 0, is_running: false }] });
    expect(stepDone(s, "run")).toBe(false);
    expect(firstIncompleteStep(s)).toBe("run");
  });

  it("repeat mode counts as something queued to run", () => {
    const s = makeSummary({ instances: [{ id: 1, name: "S1", run_mode: "repeat_event", queue_pending: 0, is_running: true }] });
    expect(stepDone(s, "run")).toBe(true);
  });

  it("port conflict fails the instance step", () => {
    const s = makeSummary({ port_conflict: "Port 9600 used twice" });
    expect(stepDone(s, "instance")).toBe(false);
  });

  it("presetsReady needs all four preset types", () => {
    expect(presetsReady(makeSummary())).toBe(true);
    expect(presetsReady(makeSummary({ presets: { difficulties: 0, sessions: 1, classes: 1, times: 1 } }))).toBe(false);
  });
});
