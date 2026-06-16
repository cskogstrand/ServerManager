// Client-only demo feed for the broadcast screen. When toggled on it fabricates
// a fake running session — a random track image plus a random grid of drivers,
// each with a random car, livery and video stream — and animates their
// telemetry so the two-column layout can be exercised without a live Assetto
// Corsa server. Nothing here touches the backend; it only produces the same
// shapes the SSE store would feed, so the page paints it identically.
import { computed, ref } from "vue";
import type { CarPositionState, DriverState, SessionState } from "@/stores/server";
import type { StreamChannel } from "@/lib/useDriverStreams";

export interface DemoMapMeta {
  width: number;
  height: number;
  x_offset: number;
  z_offset: number;
  scale_factor: number;
  margin: number;
}

const FIRST = ["Max", "Lewis", "Ana", "Kenji", "Sofia", "Diego", "Noa", "Petra", "Olivier", "Ingrid", "Mateo", "Yuki"];
const LAST = ["Halvorsen", "Tanaka", "Rossi", "Müller", "Costa", "Andersen", "Dubois", "Novak", "Berg", "Okafor", "Reyes", "Lindqvist"];
const CARS = [
  "Ferrari 488 GT3 Evo",
  "Porsche 911 GT3 R",
  "Mercedes-AMG GT3",
  "Audi R8 LMS Evo II",
  "BMW M4 GT3",
  "Lamborghini Huracán GT3",
  "McLaren 720S GT3",
  "Aston Martin Vantage GT3",
  "Nissan GT-R Nismo GT3",
  "Honda NSX GT3 Evo",
];
// Public, embeddable clips — purely to populate the video tiles in demo mode.
const STREAMS = [
  "https://www.youtube.com/embed/aqz-KE-bpKQ",
  "https://www.youtube.com/embed/jNQXAC9IVRw",
  "https://www.youtube.com/embed/BHACKCNDMW8",
  "https://www.youtube.com/embed/Bey4XXJAqS8",
  "https://www.youtube.com/embed/5qap5aO4i9A",
];

const TICK_MS = 250;

function pick<T>(arr: T[]): T {
  return arr[Math.floor(Math.random() * arr.length)];
}
function randInt(min: number, max: number): number {
  return Math.floor(min + Math.random() * (max - min + 1));
}
function randGuid(): string {
  return Array.from({ length: 17 }, () => randInt(0, 9)).join("");
}

export function useBroadcastDemo() {
  const drivers = ref<DriverState[]>([]);
  const positions = ref<CarPositionState[]>([]);
  const elapsed = ref(0);
  const seed = ref(1);

  // Per-car lap phase: a fixed offset plus a per-car advance rate. tick() walks
  // each car around an elliptical "track" and rolls the lap counter on wrap.
  let anim: Array<{ phase: number; rate: number }> = [];
  // guid → its assigned stream (some drivers are deliberately left "offline").
  let streamFor: Record<string, { url: string; online: boolean }> = {};
  let timer: ReturnType<typeof setInterval> | null = null;

  const mapMeta: DemoMapMeta = {
    width: 1200,
    height: 800,
    x_offset: 0,
    z_offset: 0,
    scale_factor: 1,
    margin: 0,
  };
  const mapImageUrl = computed(() => `https://picsum.photos/seed/track-${seed.value}/1200/800`);

  function tick() {
    elapsed.value += TICK_MS;
    positions.value = drivers.value.map((d, i) => {
      const a = anim[i];
      const prev = a.phase;
      a.phase = (a.phase + a.rate) % 1;
      if (a.phase < prev) {
        // Lap completed: bump the counter and post a fresh "last lap".
        d.laps += 1;
        d.last_lap_ms = d.best_lap_ms + randInt(0, 3500);
      }
      const ang = a.phase * Math.PI * 2;
      const x = 600 + Math.cos(ang) * 470 + Math.sin(ang * 2) * 60;
      const z = 400 + Math.sin(ang) * 320;
      const speed = 32 + Math.abs(Math.sin(ang * 2)) * 76; // ~m/s
      return {
        car_id: d.car_id,
        x,
        y: 0,
        z,
        velocity_x: speed,
        velocity_y: 0,
        velocity_z: 0,
        gear: Math.min(6, 2 + Math.floor(speed / 16)),
        engine_rpm: 5500 + Math.round(Math.abs(Math.sin(ang * 3)) * 3200),
        normalized_spline_pos: a.phase,
        updated_at: elapsed.value,
      } satisfies CarPositionState;
    });
  }

  // Build a brand-new random grid. Safe to call while running.
  function regenerate() {
    seed.value = randInt(1, 99999);
    const count = randInt(4, 8);
    const usedNames = new Set<string>();
    const next: DriverState[] = [];
    anim = [];
    streamFor = {};
    for (let i = 0; i < count; i++) {
      let name = `${pick(FIRST)} ${pick(LAST)}`;
      while (usedNames.has(name)) name = `${pick(FIRST)} ${pick(LAST)}`;
      usedNames.add(name);

      const guid = randGuid();
      const best = randInt(83000, 92000);
      next.push({
        car_id: i,
        name,
        car: pick(CARS),
        skin: `livery-${randInt(1, 9999)}`,
        guid,
        laps: randInt(1, 18),
        last_lap_ms: best + randInt(0, 4000),
        best_lap_ms: best,
        connected: true,
      });
      anim.push({ phase: Math.random(), rate: 0.01 + Math.random() * 0.012 });
      // ~70% of the grid is "streaming"; the rest exercise the offline tile.
      streamFor[guid] = {
        url: `${pick(STREAMS)}?autoplay=1&mute=1&playsinline=1`,
        online: Math.random() < 0.7,
      };
    }
    drivers.value = next;
    elapsed.value = 0;
    tick();
  }

  function start() {
    if (!drivers.value.length) regenerate();
    stop();
    timer = setInterval(tick, TICK_MS);
  }
  function stop() {
    if (timer) {
      clearInterval(timer);
      timer = null;
    }
  }

  const session = computed<SessionState>(() => ({
    name: "Demo Race",
    type: 3,
    index: 0,
    current_session_index: 0,
    session_count: 1,
    track: "Demo Speedway",
    track_config: "",
    server_name: "Demo Server",
    time: 0,
    laps: 20,
    wait_time: 0,
    ambient_temp: 24,
    road_temp: 31,
    weather_graphics: "",
    elapsed_ms: elapsed.value,
  }));

  const channels = computed<StreamChannel[]>(() =>
    drivers.value
      .map((d) => {
        const s = streamFor[d.guid];
        return {
          key: `driver:${d.guid}`,
          title: d.name,
          url: s?.url ?? "",
          health: s?.online ? "live" : "offline",
          online: !!s?.online,
        } satisfies StreamChannel;
      })
      .sort((a, b) => Number(b.online) - Number(a.online) || a.title.localeCompare(b.title)),
  );

  // Deterministic random livery preview, keyed by the driver's unique skin.
  function carImageUrl(_model: string, skin: string): string {
    return `https://picsum.photos/seed/livery-${encodeURIComponent(skin || "x")}/320/180`;
  }

  return { drivers, positions, session, mapMeta, mapImageUrl, channels, carImageUrl, regenerate, start, stop };
}
