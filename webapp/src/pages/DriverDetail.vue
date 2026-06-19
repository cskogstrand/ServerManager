<script setup lang="ts">
// Driver detail — the profile + highlight reel for one driver. Hero with a
// (placeholder, uploadable) photo, headline KPIs, favourite car/track, a
// session history that reads as drift scores or lap times, the driver's live
// stream, and an auto-captured highlight reel of their biggest drift spikes.
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { getDriver, describeResult, fmtDate, fmtScore, sessionKindLabel, shortGuid, timeAgo } from "@/lib/driversApi";
import { lapTime } from "@/lib/raceTelemetry";
import { api, ApiError, csrfToken } from "@/lib/api";
import { useToastStore } from "@/stores/toast";
import { useAuthStore } from "@/stores/auth";
import { useServerStore } from "@/stores/server";
import { useContentStore } from "@/stores/content";
import type { DriverState } from "@/stores/server";
import type { DriverDetail, DriverResult, MediaItem } from "@/types/driverStats";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";
import Sparkline from "@/components/ui/Sparkline.vue";
import CountUp from "@/components/ui/CountUp.vue";
import TrackImage from "@/components/TrackImage.vue";
import MediaActions from "@/components/MediaActions.vue";

const route = useRoute();
const toast = useToastStore();
const auth = useAuthStore();
const server = useServerStore();
const content = useContentStore();

const guid = computed(() => String(route.params.guid));
const driver = ref<DriverDetail | null>(null);
const loading = ref(true);

// Live overlay for this one driver, if they're connected right now.
const live = computed<DriverState | null>(() => {
  for (const inst of server.instanceList) {
    if (!inst.running) continue;
    const d = inst.drivers.find((x) => x.connected && x.guid === guid.value);
    if (d) return d;
  }
  return null;
});
const online = computed(() => !!live.value || !!driver.value?.online);
const liveDrift = computed(() => (live.value ? live.value.drift_live || live.value.drift_best || 0 : 0));

// --- avatar upload (local preview until the backend persists it) -------------
const fileInput = ref<HTMLInputElement | null>(null);
const localAvatar = ref<string | null>(null);
const avatarSrc = computed(() => localAvatar.value ?? driver.value?.avatar_url ?? null);

function pickPhoto() {
  fileInput.value?.click();
}
async function onPickFile(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  if (localAvatar.value) URL.revokeObjectURL(localAvatar.value);
  localAvatar.value = URL.createObjectURL(file); // instant local preview
  input.value = ""; // let the same file be re-picked later
  try {
    const fd = new FormData();
    fd.append("file", file);
    // Raw fetch: the JSON api client can't send multipart. Mirror its CSRF header.
    const res = await fetch(`/api/drivers/${encodeURIComponent(guid.value)}/avatar`, {
      method: "POST",
      headers: { "X-CSRF-Token": csrfToken() },
      body: fd,
    });
    if (!res.ok) throw new Error(String(res.status));
    toast.success("Driver photo updated.");
  } catch {
    toast.info("Showing a local preview — saving will work once the updated server is running.");
  }
}

// --- "Take picture" — grab a still from the live stream now ------------------
const snapping = ref(false);

async function takePicture() {
  if (!driver.value || snapping.value) return;
  snapping.value = true;
  try {
    await api.post(`/api/drivers/${encodeURIComponent(guid.value)}/snapshot`);
    toast.success("Picture captured.");
    const fresh = await getDriver(guid.value);
    if (fresh) driver.value = fresh;
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    snapping.value = false;
  }
}

// --- manual "Record now" -----------------------------------------------------
const recording = ref(false);

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function recordNow() {
  if (!driver.value || recording.value) return;
  recording.value = true;
  try {
    const res = await fetch(`/api/drivers/${encodeURIComponent(guid.value)}/record`, {
      method: "POST",
      headers: { "X-CSRF-Token": csrfToken() },
    });
    if (!res.ok) {
      const body = await res.json().catch(() => null);
      throw new Error(body?.error?.message ?? `Request failed (${res.status})`);
    }
    toast.info("Recording — the clip ends with the next drift run, then shows up here.");
    void pollForClip();
  } catch (e) {
    recording.value = false;
    toast.error(e instanceof Error ? e.message : String(e));
  }
}

// Recording ends server-side on the next drift run; poll until the clip lands
// (or give up after ~3 min) so it appears without a manual refresh.
async function pollForClip() {
  const before = driver.value?.media.length ?? 0;
  for (let i = 0; i < 18 && recording.value; i++) {
    await sleep(10000);
    if (!recording.value) return;
    try {
      const fresh = await getDriver(guid.value);
      if (fresh) {
        driver.value = fresh;
        if (fresh.media.length > before) {
          toast.success("Clip captured.");
          break;
        }
      }
    } catch {
      /* transient — keep polling */
    }
  }
  recording.value = false;
}

// --- derived data ------------------------------------------------------------
const screenshots = computed(() => driver.value?.media.filter((m) => m.kind === "screenshot") ?? []);
const clips = computed(() => driver.value?.media.filter((m) => m.kind === "clip") ?? []);
const streamLive = computed(() => !!driver.value?.stream && driver.value.stream.status === "live" && !!driver.value.stream.embed_url);

// Resolve the favourite car's preview against the cached car list — the same
// source the Content car grid renders from — so the image matches (and carries
// the imageVersion cache-buster) even when the driver's recorded skin is blank
// or no longer present. Falls back to the recorded skin if the car isn't cached.
const carImgUrl = computed(() => {
  const c = driver.value?.favourite_car;
  if (!c) return "";
  const car = content.carByKey(c.key);
  const skin = car?.skins.find((s) => s.key === c.skin)?.key ?? car?.skins[0]?.key ?? c.skin ?? "";
  return `/api/car/image/${encodeURIComponent(c.key)}/${encodeURIComponent(skin)}?v=${content.imageVersion}`;
});
const carImgOk = ref(true);
// Give a freshly-resolved URL a clean shot (the car list may load after first paint).
watch(carImgUrl, () => {
  carImgOk.value = true;
});

const tileTints = [
  "linear-gradient(135deg, rgba(98,179,232,0.18), rgba(16,26,37,0.94))",
  "linear-gradient(135deg, rgba(150,140,232,0.16), rgba(16,26,37,0.94))",
  "linear-gradient(135deg, rgba(96,202,202,0.16), rgba(16,26,37,0.94))",
];
const tileStyle = (i: number) => ({ background: tileTints[i % tileTints.length] });

function kindBadge(r: DriverResult): string {
  if (r.kind === "drift") return "border-accent/40 bg-accent-dim text-accent";
  if (r.kind === "race" && r.position === 1) return "border-warn/45 bg-warn-glow text-warn";
  if (r.kind === "race") return "border-line-hi bg-surface-3 text-text";
  return "border-line bg-surface-2 text-muted";
}

async function load() {
  loading.value = true;
  recording.value = false;
  carImgOk.value = true;
  try {
    driver.value = await getDriver(guid.value);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
    driver.value = null;
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void content.load(); // car list backs the favourite-car preview
  void load();
});
watch(guid, load);
onBeforeUnmount(() => {
  if (localAvatar.value) URL.revokeObjectURL(localAvatar.value);
  recording.value = false;
});

// Real captures resolve to a served file; mock items use "#" and fall back to a
// styled placeholder tile.
function isRealMedia(m: MediaItem): boolean {
  return !!m.url && m.url !== "#";
}

// --- highlight download / delete ---------------------------------------------
// Both key off m.url (= /api/drivers/:guid/media/:file); download flips the
// served file to an attachment, delete removes the row + file then drops it
// from the local list so the reel updates without a refetch.
function downloadMedia(m: MediaItem) {
  if (!isRealMedia(m)) return;
  const href = m.url + (m.url.includes("?") ? "&" : "?") + "download=1";
  const a = document.createElement("a");
  a.href = href;
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  a.remove();
}

const deleting = ref<Set<string>>(new Set());

async function deleteMedia(m: MediaItem) {
  if (!isRealMedia(m) || deleting.value.has(m.id)) return;
  const label = m.kind === "clip" ? "clip" : "screenshot";
  if (!window.confirm(`Delete this ${label}? This can't be undone.`)) return;
  deleting.value.add(m.id);
  try {
    await api.delete(m.url);
    if (driver.value) {
      driver.value.media = driver.value.media.filter((x) => x.id !== m.id);
    }
    toast.success(`${label[0].toUpperCase()}${label.slice(1)} deleted.`);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    deleting.value.delete(m.id);
  }
}
</script>

<template>
  <RouterLink
    :to="{ name: 'drivers' }"
    class="mb-4 inline-flex items-center gap-1.5 text-sm font-semibold text-muted transition-colors hover:text-accent"
  >
    <Icon name="arrowLeft" :size="16" />
    Driver Stats
  </RouterLink>

  <!-- Loading -->
  <div v-if="loading" class="space-y-4">
    <div class="h-44 animate-pulse rounded-lg border border-line bg-surface-2/50" />
    <div class="grid gap-4 md:grid-cols-2">
      <div class="h-40 animate-pulse rounded-md border border-line bg-surface-2/50" />
      <div class="h-40 animate-pulse rounded-md border border-line bg-surface-2/50" />
    </div>
  </div>

  <!-- Not found -->
  <Card v-else-if="!driver">
    <div class="py-12 text-center">
      <div class="mx-auto grid size-12 place-items-center rounded-lg border border-line bg-surface-2 text-dim">
        <Icon name="alert" :size="22" />
      </div>
      <p class="mt-3 text-sm font-semibold text-text">Driver not found</p>
      <p class="mt-0.5 text-sm text-muted">No driver with GUID <span class="font-mono">{{ shortGuid(guid) }}</span> has been seen.</p>
    </div>
  </Card>

  <template v-else>
    <!-- HERO -->
    <section class="hero reveal relative overflow-hidden rounded-lg border border-line">
      <div class="hero-grid" />
      <div class="relative flex flex-col gap-5 p-5 md:flex-row md:items-start md:p-6">
        <!-- avatar + identity -->
        <div class="flex items-center gap-4">
          <div class="relative">
            <DriverAvatar :name="driver.name" :guid="driver.guid" :src="avatarSrc" :size="137" radius="lg" />
            <button
              v-if="auth.canOperate"
              type="button"
              class="absolute -right-1.5 -bottom-1.5 grid size-7 cursor-pointer place-items-center rounded-md border border-line-hi bg-surface-3 text-muted transition-colors hover:border-accent/60 hover:text-accent"
              title="Upload photo"
              @click="pickPhoto"
            >
              <Icon name="upload" :size="14" />
            </button>
            <input ref="fileInput" type="file" accept="image/*" class="hidden" @change="onPickFile" />
          </div>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="text-2xl font-black tracking-tight text-text">{{ driver.name }}</h1>
              <span
                v-if="online"
                class="inline-flex items-center gap-1.5 rounded-full border border-ok/40 bg-ok-glow px-2 py-0.5 text-[11px] font-bold tracking-wide text-ok uppercase"
              >
                <span class="size-1.5 rounded-full bg-ok live-dot" />
                On track{{ liveDrift ? ` · ${fmtScore(liveDrift)}` : "" }}
              </span>
            </div>
            <div class="mt-1 font-mono text-xs text-dim">{{ driver.guid }}</div>
            <div class="mt-1 text-xs text-muted">
              Member since {{ fmtDate(driver.first_seen) }} · last seen {{ timeAgo(driver.last_seen) }}
            </div>
          </div>
        </div>

        <!-- KPIs + trend -->
        <div class="md:ml-auto md:max-w-[460px] md:flex-1">
          <div class="grid grid-cols-3 gap-2 sm:grid-cols-5 md:grid-cols-3 lg:grid-cols-5">
            <div class="rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Best drift</div>
              <div class="font-mono text-lg font-bold tabular-nums text-accent"><CountUp :value="driver.best_drift" /></div>
            </div>
            <div class="rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Fastest lap</div>
              <div class="font-mono text-lg font-bold tabular-nums text-text">{{ driver.best_lap_ms ? lapTime(driver.best_lap_ms) : "—" }}</div>
            </div>
            <div class="rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Laps</div>
              <div class="font-mono text-lg font-bold tabular-nums text-text"><CountUp :value="driver.total_laps" /></div>
            </div>
            <div class="rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Sessions</div>
              <div class="font-mono text-lg font-bold tabular-nums text-text"><CountUp :value="driver.sessions" /></div>
            </div>
            <div class="rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Podiums</div>
              <div class="font-mono text-lg font-bold tabular-nums" :class="driver.podiums ? 'text-warn' : 'text-text'"><CountUp :value="driver.podiums" /></div>
            </div>
          </div>
          <div class="mt-2 flex items-center gap-3 rounded-md border border-line bg-surface/70 px-3 py-2 text-accent">
            <span class="text-[10px] font-bold tracking-wide text-dim uppercase">Drift trend</span>
            <Sparkline :values="driver.drift_trend" :width="300" :height="34" class="ml-auto" />
          </div>
        </div>
      </div>
    </section>

    <!-- FAVOURITES -->
    <div class="mt-4 grid gap-4 md:grid-cols-2">
      <Card>
        <template #header>
          <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
            <Icon name="star" :size="15" class="text-warn" /> Favourite car
          </h2>
        </template>
        <div v-if="driver.favourite_car" class="flex items-center gap-4">
          <div class="grid h-20 w-32 shrink-0 place-items-center overflow-hidden rounded-md border border-line bg-surface-2">
            <img
              v-if="carImgOk"
              :src="carImgUrl"
              alt=""
              class="size-full object-cover"
              @error="carImgOk = false"
            />
            <Icon v-else name="car" :size="28" class="text-dim" />
          </div>
          <div class="min-w-0">
            <div class="truncate text-base font-bold text-text">{{ driver.favourite_car.name }}</div>
            <div v-if="driver.favourite_car.skin" class="truncate text-xs text-muted">Livery · {{ driver.favourite_car.skin }}</div>
            <div class="mt-1 text-[11px] font-semibold tracking-wide text-dim uppercase">Most-driven car</div>
          </div>
        </div>
        <p v-else class="text-sm text-muted">No car data yet.</p>
      </Card>

      <Card>
        <template #header>
          <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
            <Icon name="mapPin" :size="15" class="text-accent" /> Favourite track
          </h2>
        </template>
        <div v-if="driver.favourite_track" class="flex items-center gap-4">
          <TrackImage
            :track-key="driver.favourite_track.key"
            :config="driver.favourite_track.config"
            class="h-20 w-32 shrink-0 rounded-md border border-line bg-surface-2"
          />
          <div class="min-w-0">
            <div class="truncate text-base font-bold text-text">{{ driver.favourite_track.name }}</div>
            <div v-if="driver.favourite_track.country" class="truncate text-xs text-muted">{{ driver.favourite_track.country }}</div>
            <div class="mt-1 text-[11px] font-semibold tracking-wide text-dim uppercase">Most-raced layout</div>
          </div>
        </div>
        <p v-else class="text-sm text-muted">No track data yet.</p>
      </Card>
    </div>

    <!-- HISTORY + STREAM -->
    <div class="mt-4 grid items-start gap-4 lg:grid-cols-[1fr_380px]">
      <Card>
        <template #header>
          <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
            <Icon name="activity" :size="15" /> Recent sessions
          </h2>
          <span class="text-xs text-dim">drift & timed</span>
        </template>
        <ul v-if="driver.results.length" class="-my-1">
          <li
            v-for="(r, i) in driver.results"
            :key="r.session_id"
            class="reveal flex items-center gap-3 border-b border-line/60 py-2.5 last:border-0"
            :style="{ animationDelay: Math.min(i, 12) * 35 + 'ms' }"
          >
            <span
              class="grid w-16 shrink-0 place-items-center rounded-md border py-1 text-[10px] font-bold tracking-wide uppercase"
              :class="kindBadge(r)"
            >
              {{ sessionKindLabel[r.kind] }}
            </span>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-semibold text-text">{{ r.track.name }}</div>
              <div class="truncate text-xs text-dim">
                <Icon name="car" :size="12" class="-mt-px mr-1 inline" />{{ r.car.name }} · {{ fmtDate(r.date) }}
              </div>
            </div>
            <div class="shrink-0 text-right">
              <div
                class="font-mono text-sm font-bold tabular-nums"
                :class="describeResult(r).tone === 'warn' ? 'text-warn' : describeResult(r).tone === 'accent' ? 'text-accent' : 'text-text'"
              >
                {{ describeResult(r).primary }}
              </div>
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">{{ describeResult(r).unit }}</div>
            </div>
          </li>
        </ul>
        <p v-else class="py-6 text-center text-sm text-muted">No sessions recorded yet.</p>
      </Card>

      <Card>
        <template #header>
          <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
            <Icon name="broadcast" :size="15" :class="streamLive ? 'text-ok' : 'text-dim'" /> Live stream
          </h2>
          <span
            v-if="streamLive"
            class="inline-flex items-center gap-1.5 rounded-full border border-ok/40 bg-ok-glow px-2 py-0.5 text-[10px] font-bold tracking-wide text-ok uppercase"
          >
            <span class="size-1.5 rounded-full bg-ok live-dot" /> Live
          </span>
        </template>
        <template v-if="auth.canOperate" #actions>
          <Button
            size="sm"
            variant="ghost"
            :disabled="snapping"
            title="Grab a still from the live stream now"
            @click="takePicture"
          >
            <Icon name="camera" :size="14" />
            {{ snapping ? "Capturing…" : "Take picture" }}
          </Button>
          <Button
            size="sm"
            :variant="recording ? 'danger' : 'ghost'"
            :disabled="recording"
            title="Record a clip that ends when the current or next drift run ends"
            @click="recordNow"
          >
            <Icon :name="recording ? 'activity' : 'record'" :size="14" />
            {{ recording ? "Recording…" : "Record now" }}
          </Button>
        </template>
        <iframe
          v-if="streamLive"
          :src="driver.stream!.embed_url"
          class="aspect-video w-full rounded-md border border-line"
          allow="autoplay; encrypted-media; picture-in-picture"
          allowfullscreen
          referrerpolicy="strict-origin-when-cross-origin"
        />
        <div
          v-else
          class="grid aspect-video place-items-center rounded-md border border-dashed border-line bg-surface-2/40 text-center"
        >
          <div class="px-4">
            <Icon name="broadcast" :size="26" class="mx-auto text-dim" />
            <p class="mt-2 text-sm font-semibold text-text">
              {{ driver.stream?.status === "offline" ? "Stream offline" : "No stream configured" }}
            </p>
            <p class="mt-0.5 text-xs text-muted">
              {{ driver.stream?.status === "offline" ? "The driver's stream isn't live right now." : "Add a stream URL under Instances → Driver streams." }}
            </p>
          </div>
        </div>
      </Card>
    </div>

    <!-- HIGHLIGHT REEL -->
    <Card class="mt-4">
      <template #header>
        <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
          <Icon name="film" :size="15" class="text-accent" /> Stream highlights
        </h2>
        <span class="hidden text-xs text-dim sm:inline">auto-captured on big drift spikes</span>
      </template>

      <div v-if="driver.media.length" class="space-y-5">
        <div v-if="clips.length">
          <div class="mb-2 flex items-center gap-2 text-[11px] font-bold tracking-wide text-dim uppercase">
            <Icon name="film" :size="13" /> Clips <span class="text-dim/70">({{ clips.length }})</span>
          </div>
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <article
              v-for="(m, i) in clips"
              :key="m.id"
              class="reveal group overflow-hidden rounded-md border border-line bg-surface-2/40"
              :style="{ animationDelay: i * 40 + 'ms' }"
            >
              <div class="relative aspect-video overflow-hidden" :style="!isRealMedia(m) ? tileStyle(i) : undefined">
                <video
                  v-if="isRealMedia(m)"
                  :src="m.url"
                  class="size-full bg-black object-cover"
                  preload="none"
                  controls
                  playsinline
                />
                <template v-else>
                  <div class="scanlines" />
                  <div class="absolute inset-0 grid place-items-center">
                    <span class="grid size-11 place-items-center rounded-full border border-text/20 bg-bg/40 text-text/85 backdrop-blur-sm">
                      <Icon name="play" :size="20" />
                    </span>
                  </div>
                </template>
                <span
                  v-if="m.trigger"
                  class="pointer-events-none absolute top-2 left-2 inline-flex items-center gap-1 rounded-md border border-warn/45 bg-warn-glow px-1.5 py-0.5 font-mono text-[10px] font-bold text-warn"
                >
                  <Icon name="arrowUp" :size="11" /> +{{ fmtScore(m.trigger.delta) }}
                </span>
                <span v-if="m.duration_s" class="pointer-events-none absolute top-2 right-2 rounded bg-bg/60 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-text/90">
                  0:{{ String(m.duration_s).padStart(2, "0") }}
                </span>
                <MediaActions v-if="isRealMedia(m)" :item="m" :can-delete="auth.canOperate" :deleting="deleting.has(m.id)" @download="downloadMedia(m)" @delete="deleteMedia(m)" />
              </div>
              <div class="px-2.5 py-1.5">
                <div class="truncate text-xs font-semibold text-text">{{ m.caption }}</div>
                <div class="text-[10px] text-muted">{{ timeAgo(m.captured_at) }}</div>
              </div>
            </article>
          </div>
        </div>

        <div v-if="screenshots.length">
          <div class="mb-2 flex items-center gap-2 text-[11px] font-bold tracking-wide text-dim uppercase">
            <Icon name="camera" :size="13" /> Screenshots <span class="text-dim/70">({{ screenshots.length }})</span>
          </div>
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <article
              v-for="(m, i) in screenshots"
              :key="m.id"
              class="reveal group overflow-hidden rounded-md border border-line bg-surface-2/40"
              :style="{ animationDelay: i * 40 + 'ms' }"
            >
              <div class="relative aspect-video overflow-hidden" :style="!isRealMedia(m) ? tileStyle(i + 1) : undefined">
                <img
                  v-if="isRealMedia(m)"
                  :src="m.url"
                  alt=""
                  loading="lazy"
                  class="size-full bg-black object-cover"
                />
                <template v-else>
                  <div class="scanlines" />
                  <div class="absolute inset-0 grid place-items-center text-text/30">
                    <Icon name="camera" :size="22" />
                  </div>
                </template>
                <span
                  v-if="m.trigger"
                  class="pointer-events-none absolute top-2 left-2 inline-flex items-center gap-1 rounded-md border border-warn/45 bg-warn-glow px-1.5 py-0.5 font-mono text-[10px] font-bold text-warn"
                >
                  <Icon name="arrowUp" :size="11" /> +{{ fmtScore(m.trigger.delta) }}
                </span>
                <MediaActions v-if="isRealMedia(m)" :item="m" :can-delete="auth.canOperate" :deleting="deleting.has(m.id)" @download="downloadMedia(m)" @delete="deleteMedia(m)" />
              </div>
              <div class="px-2.5 py-1.5">
                <div class="truncate text-[11px] font-semibold text-text">{{ m.caption }}</div>
                <div class="text-[10px] text-muted">{{ timeAgo(m.captured_at) }}</div>
              </div>
            </article>
          </div>
        </div>
      </div>

      <div v-else class="py-10 text-center">
        <div class="mx-auto grid size-12 place-items-center rounded-lg border border-line bg-surface-2 text-dim">
          <Icon name="camera" :size="22" />
        </div>
        <p class="mt-3 text-sm font-semibold text-text">No highlights yet</p>
        <p class="mx-auto mt-0.5 max-w-md text-sm text-muted">
          Screenshots and clips are captured automatically from this driver's stream when they land a big drift
          spike. Nothing has tripped the trigger yet.
        </p>
      </div>
    </Card>
  </template>
</template>

<style scoped>
.hero {
  background:
    radial-gradient(130% 150% at 0% 0%, rgba(98, 179, 232, 0.12), transparent 55%),
    linear-gradient(180deg, var(--color-surface-2), var(--color-surface));
}
.hero-grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    linear-gradient(rgba(53, 80, 106, 0.18) 1px, transparent 1px),
    linear-gradient(90deg, rgba(53, 80, 106, 0.18) 1px, transparent 1px);
  background-size: 28px 28px;
  mask-image: radial-gradient(120% 120% at 100% 0%, #000 0%, transparent 70%);
}
.scanlines {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: repeating-linear-gradient(0deg, rgba(255, 255, 255, 0.035) 0 1px, transparent 1px 3px);
}
@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
.reveal {
  animation: rise 0.5s cubic-bezier(0.22, 0.61, 0.36, 1) both;
}
@keyframes ping-soft {
  0% {
    box-shadow: 0 0 0 0 rgba(79, 216, 132, 0.5);
  }
  70%,
  100% {
    box-shadow: 0 0 0 5px rgba(79, 216, 132, 0);
  }
}
.live-dot {
  animation: ping-soft 1.8s ease-out infinite;
}
</style>
