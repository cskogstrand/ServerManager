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
import Modal from "@/components/ui/Modal.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";
import Sparkline from "@/components/ui/Sparkline.vue";
import DriverFavourites from "@/components/DriverFavourites.vue";
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
    if (!res.ok) { const body = await res.json().catch(() => null); throw new Error(body?.error?.message || `Photo upload failed (${res.status})`); }
    toast.success("Driver photo updated.");
  } catch (e) {
    if (localAvatar.value) URL.revokeObjectURL(localAvatar.value);
    localAvatar.value = null;
    toast.error(String(e));
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

// Deep-link target: /drivers/:guid?media=:file opens that clip/still in a
// lightbox (used by the feed's "Clip saved" link). The query carries the file
// basename, which is the last segment of every media url.
const activeMedia = ref<MediaItem | null>(null);
const focusMediaFile = computed(() => (typeof route.query.media === "string" ? route.query.media : ""));
function mediaFile(m: MediaItem): string {
  return (m.url.split("?")[0].split("/").pop() ?? "");
}
function openDeepLinkedMedia() {
  const d = driver.value;
  if (!focusMediaFile.value || !d) return;
  const all = [...d.media, ...d.session_history.flatMap((s) => [...s.media, ...s.drift_runs.map((r) => r.clip)])];
  const m = all.find((x): x is MediaItem => !!x && isRealMedia(x) && mediaFile(x) === focusMediaFile.value);
  if (m) activeMedia.value = m;
}
// Reopen when navigating between feed media links while already on this page
// (guid unchanged, so load() — which fires the initial open — won't re-run).
watch(focusMediaFile, openDeepLinkedMedia);

async function load() {
  loading.value = true;
  recording.value = false;
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
  // Deep-link: pop the requested clip/still in its lightbox.
  openDeepLinkedMedia();
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
  const ok = await confirm.ask({
    title: `Delete ${label}`,
    message: `Delete this ${label}?`,
    detail: "This cannot be undone.",
    confirmLabel: `Delete ${label}`,
    tone: "danger",
  });
  if (!ok) return;
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
// returned set. Every failure restores the saved state.
async function addTag(session: DriverSession, tag: string) {
  if (session.tags.some((t) => t.toLowerCase() === tag.toLowerCase())) return;
  const prev = session.tags.slice();
  session.tags = [...session.tags, tag];
  try {
    session.tags = await addSessionTag(guid.value, session.id, tag);
  } catch (e) {
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
    if (driver.value) driver.value.session_history = prev;
    toast.error(e instanceof ApiError ? e.message : String(e));
  }
}
</script>

<template>
  <RouterLink
    :to="{ name: 'drivers' }"
    class="pitlane-back"
  >
    <Icon name="arrowLeft" :size="16" />
    Everyone at the club
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
    <header class="pitlane-profile-heading">
      <div class="pitlane-profile-photo">
        <DriverAvatar :name="driver.name" :guid="driver.guid" :src="avatarSrc" :size="104" />
        <button v-if="auth.canOperate" type="button" class="pitlane-photo-edit" title="Upload photo" aria-label="Upload driver photo" @click="pickPhoto"><Icon name="upload" :size="18" /></button>
        <input ref="fileInput" type="file" accept="image/*" class="hidden" @change="onPickFile" />
      </div>
      <div class="min-w-0"><span class="pitlane-eyebrow">{{ online ? 'On track now' : 'Your drivers' }}</span>
          <h1>{{ driver.name }}</h1>
          <p>Connected through Assetto Corsa · First joined {{ fmtDate(driver.first_seen) }}</p>
          <p>Last seen {{ timeAgo(driver.last_seen) }}<span v-if="online && liveDrift"> · {{ fmtScore(liveDrift) }} live drift points</span></p>
          <details class="pitlane-account-id"><summary>Account details</summary><span>{{ driver.guid }}</span></details></div>
    </header>
    <dl class="pitlane-metrics" aria-label="Driver statistics">
      <div><dt>Best drift</dt><dd>{{ driver.best_drift ? fmtScore(driver.best_drift) : '—' }}</dd></div>
      <div><dt>Fastest lap</dt><dd>{{ driver.best_lap_ms ? lapTime(driver.best_lap_ms) : '—' }}</dd></div>
      <div><dt>Laps completed</dt><dd>{{ driver.total_laps.toLocaleString() }}</dd></div>
      <div><dt>Sessions with the club</dt><dd>{{ driver.sessions.toLocaleString() }}</dd></div>
      <div><dt>Podiums</dt><dd>{{ driver.podiums.toLocaleString() }}</dd></div>
    </dl>
    <div v-if="driver.drift_trend.length" class="pitlane-profile-trend"><span>Recent drift runs</span><Sparkline :values="driver.drift_trend" :width="300" :height="34" class="text-accent max-w-full" /></div>

    <section v-if="online && live" class="profile-current">
      <div class="profile-section-heading">
        <div><span class="profile-status">On track now</span><h2>{{ sessionTypeLabel(liveSession?.type) }}<span v-if="liveSession?.name"> · {{ liveSession.name }}</span></h2><p>{{ liveTrackName || 'Current session' }}<span v-if="liveInstance"> · {{ liveInstance.name }}</span></p></div>
        <RouterLink v-if="liveInstance" :to="{ name: 'server-broadcast', params: { id: liveInstance.id } }" class="pitlane-button">Watch live <Icon name="arrowLeft" :size="17" class="rotate-180" /></RouterLink>
      </div>
      <dl class="pitlane-metrics">
        <template v-if="liveIsDrift">
          <div><dt>Live drift</dt><dd>{{ liveDrift ? fmtScore(liveDrift) : '—' }}</dd></div>
          <div><dt>Best run</dt><dd>{{ live.drift_best ? fmtScore(live.drift_best) : '—' }}</dd></div>
          <div><dt>Last run</dt><dd>{{ live.drift_last ? fmtScore(live.drift_last) : '—' }}</dd></div>
        </template>
        <template v-else>
          <div><dt>Position</dt><dd>{{ liveRow ? `P${liveRow.position}` : '—' }}</dd></div>
          <div><dt>Last lap</dt><dd>{{ live.last_lap_ms ? lapTime(live.last_lap_ms) : '—' }}</dd></div>
          <div><dt>Laps</dt><dd>{{ live.laps }}</dd></div>
        </template>
      </dl>
    </section>

    <DriverFavourites :car="driver.favourite_car" :track="driver.favourite_track" />

    <div class="profile-story-grid">
      <section class="min-w-0" aria-labelledby="recent-drives-title">
        <div class="profile-section-heading">
          <div><h2 id="recent-drives-title">Recent drives</h2><p>Laps, runs, and moments from each visit.</p></div>
          <RouterLink :to="{ name: 'session-search' }" class="profile-text-link">Find a drive <Icon name="arrowLeft" :size="17" class="rotate-180" /></RouterLink>
        </div>
        <div class="profile-drive-list">
          <SessionCard
            v-for="s in driver.session_history"
            :key="s.id"
            :session="s"
            :can-operate="auth.canOperate"
            :default-open="sessionOpen(s)"
            :deleting-ids="deleting"
            :guests="guests"
            @add-tag="(t) => addTag(s, t)"
            @remove-tag="(t) => removeTag(s, t)"
            @assign="(gid) => assignSessionGuest(s, gid)"
            @delete-media="deleteMedia"
            @download-media="downloadMedia"
            @delete-session="removeSession(s)"
          />
        </div>
        <div v-if="!driver.session_history.length" class="profile-quiet-empty">
          <h3>The first drive is still to come.</h3>
          <p>When this driver joins a server, their laps, drift runs and highlights will be kept here.</p>
        </div>
      </section>

      <aside class="profile-aside">
        <ManualRecordings
          v-if="driver.media.length"
          :items="driver.media"
          heading="Personal highlights"
          context="Saved outside a drive. More moments live in the drive history."
          :can-operate="auth.canOperate"
          :deleting-ids="deleting"
          :guests="guests"
          @assign="assignMediaGuest"
          @delete-media="deleteMedia"
          @download-media="downloadMedia"
        />
        <section v-else>
          <div class="profile-section-heading"><h2>Personal highlights</h2></div>
          <div class="profile-quiet-empty"><h3>The next good moment is out there.</h3><p>Saved clips and pictures will appear here. Moments captured during a drive stay with its history.</p></div>
        </section>

        <section class="profile-camera" aria-labelledby="driver-camera-title">
          <div class="profile-section-heading">
            <h2 id="driver-camera-title">From the simulator</h2>
            <span v-if="streamLive" class="profile-status">Live</span>
          </div>
          <div v-if="streamLive" class="profile-camera-player">
            <WhepPlayer v-if="isWhepUrl(driver.stream!.embed_url)" :url="driver.stream!.embed_url" />
            <iframe v-else :src="driver.stream!.embed_url" title="Driver live camera" class="size-full" allow="autoplay; encrypted-media; picture-in-picture" allowfullscreen referrerpolicy="strict-origin-when-cross-origin" />
          </div>
          <div v-else class="profile-quiet-empty">
            <h3>{{ driver.stream?.status === 'offline' ? 'The camera is taking a break.' : 'A view from their seat.' }}</h3>
            <p>{{ driver.stream?.status === 'offline' ? 'The configured stream is offline right now.' : 'No stream is configured for this account yet.' }}</p>
            <RouterLink v-if="auth.canOperate" to="/garage/rigs" class="profile-text-link">Rigs & cameras <Icon name="arrowLeft" :size="17" class="rotate-180" /></RouterLink>
          </div>
          <p v-if="capStatus?.manual_active" class="profile-recorder-state text-danger">Recording · {{ fmtClipDuration(cap.manualElapsed(guid)) }}</p>
          <p v-else-if="capStatus?.buffering" class="profile-recorder-state">Ready to capture · Receiving video</p>
          <p v-else-if="capStatus?.recorder_running" class="profile-recorder-state">Recorder connecting · Waiting for video</p>
          <div v-if="auth.canOperate" class="pitlane-actions mt-4">
            <Button size="sm" variant="ghost" :disabled="snapping" title="Grab a still from the live stream now" @click="takePicture">
              <Icon name="camera" :size="16" /> {{ snapping ? 'Capturing…' : 'Take picture' }}
            </Button>
            <Button size="sm" :variant="isRecording ? 'danger' : 'ghost'" :title="isRecording ? 'Stop recording now and save the clip' : 'Record a clip that ends when the current or next drift run ends'" @click="isRecording ? stopRecord() : recordNow()">
              <Icon :name="isRecording ? 'stop' : 'record'" :size="16" /> {{ isRecording ? 'Stop recording' : 'Record now' }}
            </Button>
          </div>
        </section>
      </aside>
    </div>

    <!-- Deep-link lightbox: plays/shows the media named in ?media=:file -->
    <Modal
      :open="!!activeMedia"
      wide
      :title="activeMedia?.caption || (activeMedia?.kind === 'clip' ? 'Clip' : 'Capture')"
      @close="activeMedia = null"
    >
      <video
        v-if="activeMedia?.kind === 'clip'"
        :src="activeMedia.url"
        class="mx-auto max-h-[78vh] w-full rounded-md bg-black object-contain"
        controls
        autoplay
        playsinline
      />
      <img v-else-if="activeMedia" :src="activeMedia.url" :alt="activeMedia.caption" class="mx-auto max-h-[78vh] w-full rounded-md object-contain" />
      <template #footer>
        <Button v-if="activeMedia && isRealMedia(activeMedia)" variant="ghost" @click="downloadMedia(activeMedia)">
          <Icon name="download" :size="14" class="mr-1.5" /> Download
        </Button>
        <Button @click="activeMedia = null">Close</Button>
      </template>
    </Modal>
  </template>
</template>
