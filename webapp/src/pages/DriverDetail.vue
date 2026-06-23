<script setup lang="ts">
// Driver detail — the profile + highlight reel for one driver. Hero with a
// (placeholder, uploadable) photo, headline KPIs, favourite car/track, a
// session history that reads as drift scores or lap times, the driver's live
// stream, and an auto-captured highlight reel of their biggest drift spikes.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { getDriver, addSessionTag, removeSessionTag, deleteSession, fmtDate, fmtScore, shortGuid, timeAgo } from "@/lib/driversApi";
import { listGuestDrivers, assignSession, assignMedia, type GuestDriver } from "@/lib/guestDriversApi";
import { useDriverCapture, fmtClipDuration } from "@/lib/useDriverCapture";
import { lapTime, sessionTypeLabel, computeRunningOrder } from "@/lib/raceTelemetry";
import { api, ApiError, csrfToken } from "@/lib/api";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import { useAuthStore } from "@/stores/auth";
import { useServerStore } from "@/stores/server";
import { useContentStore } from "@/stores/content";
import { useFeedStore } from "@/stores/feed";
import type { DriverState, InstanceState } from "@/stores/server";
import type { DriverDetail, DriverSession, MediaItem } from "@/types/driverStats";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";
import Sparkline from "@/components/ui/Sparkline.vue";
import CountUp from "@/components/ui/CountUp.vue";
import TrackImage from "@/components/TrackImage.vue";
import SessionCard from "@/components/SessionCard.vue";
import ManualRecordings from "@/components/ManualRecordings.vue";
import WhepPlayer from "@/components/WhepPlayer.vue";
import { isWhepUrl } from "@/lib/useDriverStreams";

const route = useRoute();
const toast = useToastStore();
const confirm = useConfirmStore();
const auth = useAuthStore();
const server = useServerStore();
const content = useContentStore();
const feed = useFeedStore();
const cap = useDriverCapture();

const guid = computed(() => String(route.params.guid));
const capStatus = computed(() => cap.statusFor(guid.value));
const driver = ref<DriverDetail | null>(null);
const loading = ref(true);
// Guest-driver roster, for the per-session "Driven by" assignment control.
const guests = ref<GuestDriver[]>([]);

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

// The running instance this driver is connected to right now (for the live
// "Current race" panel), plus derived session/standing info.
const liveInstance = computed<InstanceState | null>(() => {
  for (const inst of server.instanceList) {
    if (!inst.running) continue;
    if (inst.drivers.some((x) => x.connected && x.guid === guid.value)) return inst;
  }
  return null;
});
const liveSession = computed(() => liveInstance.value?.session ?? null);
const liveIsDrift = computed(() => !!liveInstance.value?.drift_score_enabled);
const liveRow = computed(() => {
  const inst = liveInstance.value;
  if (!inst || !live.value) return null;
  const rows = computeRunningOrder(inst.drivers, inst.positions, inst.session?.type ?? 0);
  return rows.find((r) => r.car_id === live.value!.car_id) ?? null;
});
const liveTrackName = computed(() => {
  const s = liveSession.value;
  if (!s?.track) return "";
  const t =
    content.tracks.find((x) => x.key === s.track && (x.config ?? "") === (s.track_config ?? "")) ??
    content.tracks.find((x) => x.key === s.track);
  return t?.name || s.track;
});

// --- avatar upload (local preview until the backend persists it) -------------
const fileInput = ref<HTMLInputElement | null>(null);
const localAvatar = ref<string | null>(null);
const avatarSrc = computed(() => localAvatar.value ?? driver.value?.avatar_url ?? null);

const previewFallbackMsg = "Showing a local preview — saving will work once the updated server is running.";

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
    if (res.ok) toast.success("Driver photo updated.");
    else toast.info(previewFallbackMsg);
  } catch {
    toast.info(previewFallbackMsg);
  }
}

// --- "Take picture" — grab a still from the live stream now ------------------
const snapping = ref(false);

async function takePicture() {
  if (!driver.value || snapping.value) return;
  snapping.value = true;
  try {
    await cap.takePicture(guid.value);
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
    await cap.recordNow(guid.value);
    toast.info("Recording — the clip ends with the next drift run, then shows up here.");
    void pollForClip();
  } catch (e) {
    recording.value = false;
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

// True while a manual recording is in flight — locally (just pressed) or per
// the server's live capture status.
const isRecording = computed(() => recording.value || cap.isRecording(guid.value));

async function stopRecord() {
  try {
    await cap.stopRecording(guid.value);
    recording.value = false;
    toast.success("Recording stopped — clip saved.");
    const fresh = await getDriver(guid.value);
    if (fresh) driver.value = fresh;
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

// Recording ends server-side on the next drift run; poll until the clip lands
// (or give up after ~3 min) so it appears without a manual refresh.
// Total media across the manual reel + every session, so the poll notices a new
// clip whether it lands standalone (recorded off-session) or inside a session.
function mediaCount(d: DriverDetail | null): number {
  if (!d) return 0;
  return d.media.length + d.session_history.reduce((n, s) => n + s.media.length, 0);
}

async function pollForClip() {
  const before = mediaCount(driver.value);
  for (let i = 0; i < 18 && recording.value; i++) {
    await sleep(10000);
    if (!recording.value) return;
    try {
      const fresh = await getDriver(guid.value);
      if (fresh) {
        driver.value = fresh;
        if (mediaCount(fresh) > before) {
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
const streamLive = computed(() => !!driver.value?.stream && driver.value.stream.status === "live" && !!driver.value.stream.embed_url);

// Deep-link target: /drivers/:guid?session=:id opens (and scrolls to) that
// session. Otherwise every session starts collapsed (accordion style).
const focusSessionId = computed(() => (typeof route.query.session === "string" ? route.query.session : ""));
function sessionOpen(s: DriverSession): boolean {
  return !!focusSessionId.value && s.id === focusSessionId.value;
}

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
  // Deep-link: scroll the focused session into view once it's rendered.
  if (focusSessionId.value) {
    await nextTick();
    document.getElementById(`session-${focusSessionId.value}`)?.scrollIntoView({ behavior: "smooth", block: "start" });
  }
}

onMounted(() => {
  void content.load(); // car list backs the favourite-car preview
  void load();
  void listGuestDrivers().then((g) => (guests.value = g)).catch(() => {});
  cap.startPoll();
});
watch(guid, load);

// Live refresh: when a streamed event for THIS driver lands (lap, drift run,
// session start/end, saved media, recording), refetch silently — no skeleton
// flash — so the page updates without a manual refresh. Debounced so a burst
// (lap after lap) collapses into one refetch.
let liveRefreshTimer: ReturnType<typeof setTimeout> | null = null;
async function reloadSilent() {
  try {
    const fresh = await getDriver(guid.value);
    if (fresh) driver.value = fresh;
  } catch {
    /* transient — keep current data until the next event */
  }
}
watch(
  () => feed.lastId,
  () => {
    const it = feed.items[0];
    if (!it || it.guid !== guid.value || liveRefreshTimer) return;
    liveRefreshTimer = setTimeout(() => {
      liveRefreshTimer = null;
      void reloadSilent();
    }, 1200);
  },
);

onBeforeUnmount(() => {
  if (localAvatar.value) URL.revokeObjectURL(localAvatar.value);
  if (liveRefreshTimer) clearTimeout(liveRefreshTimer);
  recording.value = false;
  cap.stopPoll();
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
      // Drop it from every session group + unlink any drift run that showed it.
      for (const s of driver.value.session_history) {
        s.media = s.media.filter((x) => x.id !== m.id);
        for (const run of s.drift_runs) {
          if (run.clip?.id === m.id) run.clip = null;
        }
      }
    }
    toast.success(`${label[0].toUpperCase()}${label.slice(1)} deleted.`);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    deleting.value.delete(m.id);
  }
}

// --- session tags ------------------------------------------------------------
// Optimistic: update the chip set immediately, reconcile with the server's
// returned set. In mock/preview mode (endpoint 404/501) keep the optimistic
// change so the UI stays usable.
async function addTag(session: DriverSession, tag: string) {
  if (session.tags.some((t) => t.toLowerCase() === tag.toLowerCase())) return;
  const prev = session.tags.slice();
  session.tags = [...session.tags, tag];
  try {
    session.tags = await addSessionTag(guid.value, session.id, tag);
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 501)) {
      toast.info(previewFallbackMsg);
      return;
    }
    session.tags = prev;
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

async function removeTag(session: DriverSession, tag: string) {
  const prev = session.tags.slice();
  session.tags = session.tags.filter((t) => t !== tag);
  try {
    session.tags = await removeSessionTag(guid.value, session.id, tag);
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 501)) {
      toast.info(previewFallbackMsg);
      return;
    }
    session.tags = prev;
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

// Attribute the whole session (connection) to a guest driver, or null to revert
// it to this account's own name. Optimistic; rolls back on error.
async function assignSessionGuest(session: DriverSession, guestDriverId: number | null) {
  const prev = session.guest_driver_id ?? null;
  if (prev === guestDriverId) return;
  session.guest_driver_id = guestDriverId;
  try {
    await assignSession(guid.value, session.id, guestDriverId);
    const name = guestDriverId ? guests.value.find((g) => g.id === guestDriverId)?.name : "";
    toast.success(guestDriverId ? `Session assigned to ${name ?? "guest driver"}.` : "Session reverted to account driver.");
  } catch (e) {
    session.guest_driver_id = prev;
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

// Attribute one standalone manual recording to a guest driver (or null to
// revert). Optimistic; rolls back on error.
async function assignMediaGuest(m: MediaItem, guestDriverId: number | null) {
  if (!driver.value) return;
  const prev = m.guest_driver_id ?? null;
  if (prev === guestDriverId) return;
  m.guest_driver_id = guestDriverId;
  try {
    await assignMedia(m.url, guestDriverId);
    const name = guestDriverId ? guests.value.find((g) => g.id === guestDriverId)?.name : "";
    toast.success(guestDriverId ? `Recording assigned to ${name ?? "guest driver"}.` : "Recording reverted to account driver.");
  } catch (e) {
    m.guest_driver_id = prev;
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}

// Delete a whole connection and everything tied to it. Tally the consequences
// so the warning is concrete, confirm, then drop it from the local list.
async function removeSession(session: DriverSession) {
  const clips = session.media.length;
  const runs = session.drift_runs.length;
  const laps = session.laps.length;
  const parts = [
    runs && `${runs} drift score${runs > 1 ? "s" : ""}`,
    laps && `${laps} lap time${laps > 1 ? "s" : ""}`,
    clips && `${clips} image${clips > 1 ? "s" : ""}/video${clips > 1 ? "s" : ""}`,
  ].filter(Boolean);
  const ok = await confirm.ask({
    title: "Delete entire session?",
    message: `This permanently deletes everything from this ${session.track.name} session and can't be undone.`,
    detail: parts.length ? `Removes ${parts.join(", ")}, including the captured media files.` : "Removes all scores, lap times and captured media files.",
    confirmLabel: "Delete session",
    tone: "danger",
  });
  if (!ok || !driver.value) return;
  // Optimistic: drop it now, roll back only on a real server error.
  const prev = driver.value.session_history;
  driver.value.session_history = prev.filter((s) => s.id !== session.id);
  try {
    await deleteSession(guid.value, session.id);
    toast.success("Session deleted.");
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 501)) {
      toast.info(previewFallbackMsg);
      return;
    }
    if (driver.value) driver.value.session_history = prev;
    toast.error(e instanceof ApiError ? e.message : String(e));
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
        <div class="md:ml-auto md:max-w-125 md:flex-1">
          <div class="grid grid-cols-3 gap-2 sm:grid-cols-5 md:grid-cols-3 lg:grid-cols-5">
            <div class="rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Best drift</div>
              <div class="font-mono text-lg font-bold tabular-nums text-accent"><CountUp :value="driver.best_drift" /></div>
            </div>
            <div class="col-span-2 rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Fastest lap</div>
              <div class="truncate font-mono text-lg font-bold tabular-nums text-text">{{ driver.best_lap_ms ? lapTime(driver.best_lap_ms) : "—" }}</div>
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
            <Sparkline :values="driver.drift_trend" :width="300" :height="34" class="ml-auto h-auto min-w-0 max-w-full" />
          </div>
        </div>
      </div>
    </section>

    <!-- CURRENT RACE (only while the driver is on track) -->
    <Card v-if="online && live" class="reveal mt-4">
      <template #header>
        <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
          <Icon name="flag" :size="15" class="text-accent" /> Current race
        </h2>
        <span class="inline-flex items-center gap-1.5 rounded-full border border-ok/40 bg-ok-glow px-2 py-0.5 text-[10px] font-bold tracking-wide text-ok uppercase">
          <span class="size-1.5 rounded-full bg-ok live-dot" /> On track
        </span>
      </template>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-x-2 text-sm">
            <span class="font-bold text-text">{{ sessionTypeLabel(liveSession?.type) }}</span>
            <span v-if="liveSession?.name" class="truncate text-muted">· {{ liveSession.name }}</span>
          </div>
          <div class="mt-1 flex items-center gap-1.5 text-xs text-muted">
            <Icon name="mapPin" :size="13" class="text-dim" /> {{ liveTrackName || "—" }}
            <span v-if="liveInstance" class="text-dim">· {{ liveInstance.name }}</span>
          </div>
        </div>
        <RouterLink
          v-if="liveInstance"
          :to="{ name: 'server-broadcast', params: { id: liveInstance.id } }"
          class="inline-flex items-center gap-1.5 rounded-md border border-line bg-surface-2 px-2.5 py-1.5 text-xs font-semibold text-muted transition-colors hover:border-accent/50 hover:text-accent"
        >
          <Icon name="broadcast" :size="14" /> Broadcast
        </RouterLink>
      </div>

      <div class="mt-4 grid grid-cols-3 gap-3">
        <template v-if="liveIsDrift">
          <div class="rounded-md border border-line bg-surface-2/40 p-3 text-center">
            <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Live</div>
            <div class="numerals mt-1 text-2xl font-bold tabular-nums" :class="liveDrift ? 'text-accent' : 'text-dim'">{{ liveDrift ? fmtScore(liveDrift) : "—" }}</div>
          </div>
          <div class="rounded-md border border-line bg-surface-2/40 p-3 text-center">
            <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Best</div>
            <div class="numerals mt-1 text-2xl font-bold tabular-nums text-text">{{ live!.drift_best ? fmtScore(live!.drift_best) : "—" }}</div>
          </div>
          <div class="rounded-md border border-line bg-surface-2/40 p-3 text-center">
            <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Last</div>
            <div class="numerals mt-1 text-2xl font-medium tabular-nums text-muted">{{ live!.drift_last ? fmtScore(live!.drift_last) : "—" }}</div>
          </div>
        </template>
        <template v-else>
          <div class="rounded-md border border-line bg-surface-2/40 p-3 text-center">
            <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Position</div>
            <div class="numerals mt-1 text-2xl font-bold tabular-nums text-text">{{ liveRow ? `P${liveRow.position}` : "—" }}</div>
          </div>
          <div class="rounded-md border border-line bg-surface-2/40 p-3 text-center">
            <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Last lap</div>
            <div class="numerals mt-1 text-2xl font-medium tabular-nums text-text">{{ live!.last_lap_ms ? lapTime(live!.last_lap_ms) : "—" }}</div>
          </div>
          <div class="rounded-md border border-line bg-surface-2/40 p-3 text-center">
            <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Laps</div>
            <div class="numerals mt-1 text-2xl font-bold tabular-nums text-text">{{ live!.laps }}</div>
          </div>
        </template>
      </div>
    </Card>

    <!-- FAVOURITES -->
    <div class="mt-4 grid gap-4 md:grid-cols-2">
      <div class="min-w-0 space-y-3">
      <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
        <Icon name="star" :size="15" class="text-warn" /> Favourite car
      </h2>
      <Card class="min-w-0">
        <div v-if="driver.favourite_car" class="flex items-center gap-3 sm:gap-4">
          <div class="grid h-16 w-24 shrink-0 place-items-center overflow-hidden rounded-md border border-line bg-surface-2 sm:h-20 sm:w-32">
            <img
              v-if="carImgOk"
              :src="carImgUrl"
              alt=""
              class="size-full object-cover"
              @error="carImgOk = false"
            />
            <Icon v-else name="car" :size="28" class="text-dim" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="truncate text-base font-bold text-text">{{ driver.favourite_car.name }}</div>
            <div v-if="driver.favourite_car.skin" class="truncate text-xs text-muted">Livery · {{ driver.favourite_car.skin }}</div>
            <div class="mt-1 text-[11px] font-semibold tracking-wide text-dim uppercase">Most-driven car</div>
          </div>
        </div>
        <p v-else class="text-sm text-muted">No car data yet.</p>
      </Card>
      </div>

      <div class="min-w-0 space-y-3">
      <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
        <Icon name="mapPin" :size="15" class="text-accent" /> Favourite track
      </h2>
      <Card class="min-w-0">
        <div v-if="driver.favourite_track" class="flex items-center gap-3 sm:gap-4">
          <TrackImage
            :track-key="driver.favourite_track.key"
            :config="driver.favourite_track.config"
            class="h-16 w-24 shrink-0 rounded-md border border-line bg-surface-2 sm:h-20 sm:w-32"
          />
          <div class="min-w-0 flex-1">
            <div class="truncate text-base font-bold text-text">{{ driver.favourite_track.name }}</div>
            <div v-if="driver.favourite_track.country" class="truncate text-xs text-muted">{{ driver.favourite_track.country }}</div>
            <div class="mt-1 text-[11px] font-semibold tracking-wide text-dim uppercase">Most-raced layout</div>
          </div>
        </div>
        <p v-else class="text-sm text-muted">No track data yet.</p>
      </Card>
      </div>
    </div>

    <!-- SESSIONS + STREAM -->
    <div class="mt-4 grid items-start gap-4 lg:grid-cols-[1fr_380px]">
      <!-- Sessions: each connection (connect→disconnect) grouped together -->
      <div class="min-w-0 space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
            <Icon name="activity" :size="15" /> Sessions
            <span class="text-xs font-normal text-dim">connect → disconnect</span>
          </h2>
          <RouterLink
            :to="{ name: 'session-search' }"
            class="inline-flex items-center gap-1.5 rounded-md border border-line bg-surface-2 px-2.5 py-1.5 text-xs font-semibold text-muted transition-colors hover:border-accent/50 hover:text-accent"
          >
            <Icon name="search" :size="14" /> Search sessions
          </RouterLink>
        </div>

        <SessionCard
          v-for="(s, i) in driver.session_history"
          :key="s.id"
          :session="s"
          :can-operate="auth.canOperate"
          :default-open="sessionOpen(s)"
          :deleting-ids="deleting"
          :guests="guests"
          :style="{ animationDelay: Math.min(i, 10) * 45 + 'ms' }"
          @add-tag="(t) => addTag(s, t)"
          @remove-tag="(t) => removeTag(s, t)"
          @assign="(gid) => assignSessionGuest(s, gid)"
          @delete-media="deleteMedia"
          @download-media="downloadMedia"
          @delete-session="removeSession(s)"
        />

        <Card v-if="!driver.session_history.length">
          <div class="py-10 text-center">
            <div class="mx-auto grid size-12 place-items-center rounded-lg border border-line bg-surface-2 text-dim">
              <Icon name="activity" :size="22" />
            </div>
            <p class="mt-3 text-sm font-semibold text-text">No sessions yet</p>
            <p class="mx-auto mt-0.5 max-w-md text-sm text-muted">
              A session is one connection — from when this driver joins until they leave. Their laps, drift runs and
              highlights will be grouped here.
            </p>
          </div>
        </Card>
      </div>

      <div class="min-w-0 space-y-3 lg:sticky lg:top-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h2 class="flex flex-wrap items-center gap-2 text-sm font-bold tracking-tight">
            <Icon name="broadcast" :size="15" :class="streamLive ? 'text-ok' : 'text-dim'" /> Live stream
            <span
              v-if="streamLive"
              class="inline-flex items-center gap-1.5 rounded-full border border-ok/40 bg-ok-glow px-2 py-0.5 text-[10px] font-bold tracking-wide text-ok uppercase"
            >
              <span class="size-1.5 rounded-full bg-ok live-dot" /> Live
            </span>
            <!-- Live capture status from the rolling-buffer recorder -->
            <span
              v-if="capStatus?.manual_active"
              class="inline-flex items-center gap-1.5 rounded-full border border-danger/45 bg-danger-glow px-2 py-0.5 text-[10px] font-bold tracking-wide text-danger uppercase"
            >
              <span class="size-1.5 rounded-full bg-danger live-dot" /> REC {{ fmtClipDuration(cap.manualElapsed(guid)) }}
            </span>
            <span
              v-else-if="capStatus?.buffering"
              class="inline-flex items-center gap-1.5 rounded-full border border-ok/40 bg-ok-glow px-2 py-0.5 text-[10px] font-bold tracking-wide text-ok uppercase"
              title="Rolling buffer recording — drift-run clips are cut from this"
            >
              <span class="size-1.5 rounded-full bg-ok" /> Buffering
            </span>
            <span
              v-else-if="capStatus?.recorder_running"
              class="inline-flex items-center gap-1 rounded-full border border-warn/40 bg-warn-glow px-2 py-0.5 text-[10px] font-bold tracking-wide text-warn uppercase"
              title="Recorder running but no fresh segments — source may be down"
            >
              Connecting…
            </span>
          </h2>
          <div v-if="auth.canOperate" class="flex flex-wrap items-center gap-2">
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
              :variant="isRecording ? 'danger' : 'ghost'"
              :title="isRecording ? 'Stop recording now and save the clip' : 'Record a clip that ends when the current or next drift run ends'"
              @click="isRecording ? stopRecord() : recordNow()"
            >
              <Icon :name="isRecording ? 'stop' : 'record'" :size="14" />
              {{ isRecording ? "Stop recording" : "Record now" }}
            </Button>
          </div>
        </div>
        <Card class="min-w-0">
        <div
          v-if="streamLive && isWhepUrl(driver.stream!.embed_url)"
          class="aspect-video w-full overflow-hidden rounded-md border border-line"
        >
          <WhepPlayer :url="driver.stream!.embed_url" />
        </div>
        <iframe
          v-else-if="streamLive"
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
    </div>

    <!-- MANUAL RECORDINGS — captures made outside any session -->
    <ManualRecordings
      v-if="driver.media.length"
      class="mt-4"
      :items="driver.media"
      :can-operate="auth.canOperate"
      :deleting-ids="deleting"
      :guests="guests"
      @assign="assignMediaGuest"
      @delete-media="deleteMedia"
      @download-media="downloadMedia"
    />
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
