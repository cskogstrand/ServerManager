// Client-only demo feed for the broadcast screen. When toggled on it fabricates
// a fake running session — a random real track from the content library plus a
// random grid of drivers, each on a random real car + livery and a random video
// stream — and animates their telemetry so the two-column layout can be
// exercised without a live Assetto Corsa server. Track maps and car/livery
// previews come from the real backend endpoints (the same images the app serves
// from assetocorsa/content); only the session/telemetry are fabricated.
import { computed, ref } from "vue";
import { api } from "@/lib/api";
import type { useContentStore } from "@/stores/content";
import type { CarPositionState, DriverState, SessionState } from "@/stores/server";
import type { StreamChannel } from "@/lib/useDriverStreams";

type ContentStore = ReturnType<typeof useContentStore>;

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
// Public, embeddable clips — purely to populate the video tiles in demo mode.
const STREAMS = [
  "https://www.youtube.com/embed/4Klfc2z9WEY",
  "https://www.youtube.com/embed/6SG38HAcOkk",
  "https://www.youtube.com/embed/vFr2VqolZ34",
  "https://www.youtube.com/embed/Z8EjHfVZATU",
  "https://www.youtube.com/embed/__bfWib8iMI",
];

// Used until the real track meta resolves (and if a track has no map.ini).
const FALLBACK_META: DemoMapMeta = { width: 1200, height: 800, x_offset: 0, z_offset: 0, scale_factor: 1, margin: 0 };

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
function trackPath(kind: "map" | "mapmeta", key: string, config: string): string {
  const cfg = config ? `/${encodeURIComponent(config)}` : "";
  return `/api/track/${kind}/${encodeURIComponent(key)}${cfg}`;
}

export function useBroadcastDemo(content: ContentStore) {
  const drivers = ref<DriverState[]>([]);
  const positions = ref<CarPositionState[]>([]);
  const elapsed = ref(0);
  const track = ref<{ key: string; config: string; name: string } | null>(null);
  const mapMeta = ref<DemoMapMeta | null>(null);

  // Per-car lap phase: a fixed offset plus a per-car advance rate. tick() walks
  // each car around an ellipse mapped onto the real track and rolls the lap
  // counter on wrap.
  let anim: Array<{ phase: number; rate: number }> = [];
  // guid → its assigned stream (some drivers are deliberately left "offline").
  let streamFor: Record<string, { url: string; online: boolean }> = {};
  let timer: ReturnType<typeof setInterval> | null = null;

  const mapImageUrl = computed(() =>
    track.value ? trackPath("map", track.value.key, track.value.config) : "",
  );

  async function loadTrackMeta() {
    const t = track.value;
    if (!t) {
      mapMeta.value = null;
      return;
    }
    try {
      mapMeta.value = await api.get<DemoMapMeta>(trackPath("mapmeta", t.key, t.config));
    } catch {
      mapMeta.value = FALLBACK_META;
    }
  }

  function tick() {
    elapsed.value += TICK_MS;
    const m = mapMeta.value ?? FALLBACK_META;
    positions.value = drivers.value.map((d, i) => {
      const a = anim[i];
      const prev = a.phase;
      a.phase = (a.phase + a.rate) % 1;
      if (a.phase < prev) {
        // Lap completed: bump the counter and post a fresh "last lap".
        d.laps += 1;
        d.last_lap_ms = d.best_lap_ms + randInt(0, 3500);
      }
      // Ellipse in image-fraction space, inverted through the AC projection so
      // mapPoint() lands each puck back on that fraction of the real map.
      const th = a.phase * Math.PI * 2;
      const fx = 0.5 + 0.4 * Math.cos(th) + 0.05 * Math.cos(th * 2);
      const fy = 0.5 + 0.38 * Math.sin(th);
      const x = (fx * m.width - m.margin) * m.scale_factor - m.x_offset;
      const z = (fy * m.height - m.margin) * m.scale_factor - m.z_offset;
      const speed = 32 + Math.abs(Math.sin(th * 2)) * 76; // ~m/s
      return {
        car_id: d.car_id,
        x,
        y: 0,
        z,
        velocity_x: speed,
        velocity_y: 0,
        velocity_z: 0,
        gear: Math.min(6, 2 + Math.floor(speed / 16)),
        engine_rpm: 5500 + Math.round(Math.abs(Math.sin(th * 3)) * 3200),
        normalized_spline_pos: a.phase,
        updated_at: elapsed.value,
      } satisfies CarPositionState;
    });
  }

  // Build a brand-new random grid from the current content library. Safe to
  // call while running.
  function regenerate() {
    const tracks = (content.tracks ?? []).filter((t) => t.key);
    const cars = (content.cars ?? []).filter((c) => c.key && c.skins?.length);

    const t = tracks.length ? pick(tracks) : null;
    track.value = t ? { key: t.key!, config: t.config ?? "", name: t.name || t.key! } : null;
    mapMeta.value = t ? FALLBACK_META : null; // show the map at once; refine below
    void loadTrackMeta();

    const count = randInt(4, 8);
    const usedNames = new Set<string>();
    const next: DriverState[] = [];
    anim = [];
    streamFor = {};
    for (let i = 0; i < count; i++) {
      let name = `${pick(FIRST)} ${pick(LAST)}`;
      while (usedNames.has(name)) name = `${pick(FIRST)} ${pick(LAST)}`;
      usedNames.add(name);

      const car = cars.length ? pick(cars) : null;
      const skin = car ? pick(car.skins) : null;
      const guid = randGuid();
      const best = randInt(83000, 92000);
      next.push({
        car_id: i,
        name,
        car: car?.key ?? `demo_car_${i}`,
        skin: skin?.key ?? "",
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
    track: track.value?.name ?? "Demo Speedway",
    track_config: track.value?.config ?? "",
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

  return { drivers, positions, session, mapMeta, mapImageUrl, channels, regenerate, start, stop };
}
