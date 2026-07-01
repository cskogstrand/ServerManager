// Client-only demo feed for the broadcast screen. When toggled on it fabricates
// a fake running session — one fixed real track from the content library plus a
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
  "https://www.youtube.com/embed/4Klfc2z9WEY?autoplay=1&mute=1&loop=1&playlist=4Klfc2z9WEY&controls=0&rel=0&modestbranding=1&playsinline=1",
  "https://www.youtube.com/embed/6SG38HAcOkk?autoplay=1&mute=1&loop=1&playlist=6SG38HAcOkk&controls=0&rel=0&modestbranding=1&playsinline=1",
  "https://www.youtube.com/embed/vFr2VqolZ34?autoplay=1&mute=1&loop=1&playlist=vFr2VqolZ34&controls=0&rel=0&modestbranding=1&playsinline=1",
  "https://www.youtube.com/embed/Z8EjHfVZATU?autoplay=1&mute=1&loop=1&playlist=Z8EjHfVZATU&controls=0&rel=0&modestbranding=1&playsinline=1",
  "https://www.youtube.com/embed/__bfWib8iMI?autoplay=1&mute=1&loop=1&playlist=__bfWib8iMI&controls=0&rel=0&modestbranding=1&playsinline=1",
];

// Used until the real track meta resolves (and if a track has no map.ini).
const FALLBACK_META: DemoMapMeta = { width: 1200, height: 800, x_offset: 0, z_offset: 0, scale_factor: 1, margin: 0 };

const TICK_MS = 250;
const DEMO_TRACK = { key: "driftplayground", config: "", name: "Drift Playground" };

// Sampled from driftplayground/ai/fast_lane.ai and stored as map-image fractions.
// ponytail: static demo route; add spline parsing only if demo tracks become selectable.
const DEMO_PATH = [
  [0.8793, 0.6564],
  [0.8999, 0.6936],
  [0.9127, 0.7320],
  [0.9151, 0.7732],
  [0.9072, 0.8142],
  [0.8766, 0.8551],
  [0.8438, 0.8618],
  [0.8108, 0.8570],
  [0.7791, 0.8422],
  [0.7479, 0.8250],
  [0.7149, 0.8133],
  [0.6815, 0.8129],
  [0.6490, 0.8237],
  [0.6176, 0.8396],
  [0.5878, 0.8610],
  [0.5572, 0.8791],
  [0.5240, 0.8841],
  [0.4933, 0.8676],
  [0.4696, 0.8356],
  [0.4583, 0.7935],
  [0.4697, 0.7521],
  [0.4953, 0.7231],
  [0.5256, 0.7036],
  [0.5585, 0.6963],
  [0.5918, 0.6993],
  [0.6251, 0.7015],
  [0.6566, 0.6888],
  [0.6758, 0.6526],
  [0.6744, 0.6086],
  [0.6477, 0.5835],
  [0.6145, 0.5780],
  [0.5810, 0.5800],
  [0.5479, 0.5867],
  [0.5151, 0.5962],
  [0.4828, 0.6090],
  [0.4502, 0.6194],
  [0.4168, 0.6190],
  [0.3850, 0.6048],
  [0.3536, 0.5892],
  [0.3208, 0.5842],
  [0.2906, 0.6031],
  [0.2719, 0.6405],
  [0.2579, 0.6822],
  [0.2433, 0.7237],
  [0.2262, 0.7631],
  [0.2041, 0.7975],
  [0.1747, 0.8187],
  [0.1413, 0.8212],
  [0.1105, 0.8040],
  [0.0903, 0.7683],
  [0.0846, 0.7242],
  [0.0933, 0.6801],
  [0.1054, 0.6376],
  [0.1175, 0.5948],
  [0.1257, 0.5504],
  [0.1299, 0.5048],
  [0.1328, 0.4591],
  [0.1403, 0.4147],
  [0.1557, 0.3743],
  [0.1806, 0.3441],
  [0.2123, 0.3302],
  [0.2450, 0.3369],
  [0.2731, 0.3613],
  [0.2997, 0.3889],
  [0.3293, 0.4091],
  [0.3621, 0.4076],
  [0.3878, 0.3801],
  [0.4048, 0.3411],
  [0.4181, 0.2993],
  [0.4293, 0.2564],
  [0.4408, 0.2134],
  [0.4578, 0.1742],
  [0.4813, 0.1413],
  [0.5105, 0.1193],
  [0.5434, 0.1181],
  [0.5699, 0.1441],
  [0.5779, 0.1872],
  [0.5663, 0.2297],
  [0.5454, 0.2653],
  [0.5243, 0.3003],
  [0.5054, 0.3383],
  [0.4963, 0.3817],
  [0.5038, 0.4259],
  [0.5285, 0.4545],
  [0.5611, 0.4525],
  [0.5912, 0.4332],
  [0.6222, 0.4162],
  [0.6550, 0.4085],
  [0.6883, 0.4143],
  [0.7196, 0.4302],
  [0.7473, 0.4555],
  [0.7708, 0.4878],
  [0.7919, 0.5217],
  [0.8132, 0.5549],
  [0.8347, 0.5877],
  [0.8564, 0.6208],
] as const;

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

export function demoPathPoint(phase: number): { fx: number; fy: number } {
  const lap = ((phase % 1) + 1) % 1;
  const scaled = lap * DEMO_PATH.length;
  const i = Math.floor(scaled);
  const t = scaled - i;
  const a = DEMO_PATH[i]!;
  const b = DEMO_PATH[(i + 1) % DEMO_PATH.length]!;
  return { fx: a[0] + (b[0] - a[0]) * t, fy: a[1] + (b[1] - a[1]) * t };
}

export function demoTrackPosition(phase: number, meta: DemoMapMeta): { x: number; z: number } {
  const p = demoPathPoint(phase);
  return {
    x: (p.fx * meta.width - meta.margin) * meta.scale_factor - meta.x_offset,
    z: (p.fy * meta.height - meta.margin) * meta.scale_factor - meta.z_offset,
  };
}

export function useBroadcastDemo(content: ContentStore) {
  const drivers = ref<DriverState[]>([]);
  const positions = ref<CarPositionState[]>([]);
  const elapsed = ref(0);
  const track = ref<{ key: string; config: string; name: string } | null>(null);
  const mapMeta = ref<DemoMapMeta | null>(null);

  // Per-car lap phase: a fixed offset plus a per-car advance rate. tick() walks
  // each car around the fixed demo track route and rolls the lap
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
        // Simulate a fresh drift run so the broadcast panel animates in debug.
        const run = randInt(0, 7000);
        d.drift_last = run;
        if (run > (d.drift_best ?? 0)) d.drift_best = run;
      }
      // Live score ramps 0 → best across the lap, resetting each new run.
      d.drift_live = Math.round((d.drift_best ?? 0) * a.phase);
      const th = a.phase * Math.PI * 2;
      const { x, z } = demoTrackPosition(a.phase, m);
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
    const cars = (content.cars ?? []).filter((c) => c.key && c.skins?.length);

    const t = (content.tracks ?? []).find((item) => item.key === DEMO_TRACK.key && (item.config ?? "") === DEMO_TRACK.config);
    track.value = { ...DEMO_TRACK, name: t?.name || DEMO_TRACK.name };
    mapMeta.value = FALLBACK_META; // show the map at once; refine below
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
        drift_best: randInt(1500, 9000),
        drift_last: randInt(0, 6000),
        drift_live: randInt(0, 4000),
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
