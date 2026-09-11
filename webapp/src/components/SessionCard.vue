<script setup lang="ts">
// One driver "session" (connection: connect→disconnect) as a collapsible card.
// Header shows the stint at a glance (track, car, time range, KPIs, tags); the
// body groups the per-AC-session segments, per-lap times, drift runs + clips and
// captured media that all happened during that one connection.
import { computed, ref } from "vue";
import { lapTime } from "@/lib/raceTelemetry";
import { fmtScore, fmtDate, timeAgo, sessionKindLabel, describeResult } from "@/lib/driversApi";
import { fmtClipDuration } from "@/lib/useDriverCapture";
import { useContentStore } from "@/stores/content";
import type { DriverSession, DriverResult, MediaItem } from "@/types/driverStats";
import type { GuestDriver } from "@/lib/guestDriversApi";
import TrackImage from "@/components/TrackImage.vue";
import Icon from "@/components/ui/Icon.vue";
import MediaActions from "@/components/MediaActions.vue";
import Modal from "@/components/ui/Modal.vue";
import Button from "@/components/ui/Button.vue";

const props = defineProps<{
  session: DriverSession;
  canOperate: boolean;
  defaultOpen?: boolean;
  deletingIds?: Set<string>;
  guests?: GuestDriver[];
}>();

const emit = defineEmits<{
  (e: "add-tag", tag: string): void;
  (e: "remove-tag", tag: string): void;
  (e: "delete-media", media: MediaItem): void;
  (e: "download-media", media: MediaItem): void;
  (e: "assign", guestDriverId: number | null): void;
  (e: "delete-session"): void;
}>();

const content = useContentStore();
const isLive = computed(() => props.session.left_at === null);
const sessionTrackVersion = computed(() => content.trackByKey(props.session.track.key, props.session.track.config ?? "")?.version ?? "");
// Live sessions open by default — you want to watch them, not click into them.
const expanded = ref((props.defaultOpen ?? false) || isLive.value);
const newTag = ref("");

// A real, taggable session (id "0" is the synthetic "ungrouped" legacy bucket).
const taggable = computed(() => props.canOperate && props.session.id !== "0");

// Guest attribution: who this whole session counts for. A roster guest, or the
// GUID's own name when unassigned. Resolved against the list the parent loads.
// Reassigning is infrequent, so it lives behind a button + modal in the body.
const assignable = computed(() => props.canOperate && props.session.id !== "0");
const assignOpen = ref(false);
function pick(guestDriverId: number | null) {
  assignOpen.value = false;
  emit("assign", guestDriverId);
}

// Which kind of server this stint ran on decides what detail list shows: drift
// scores for a drift server, lap times for a race server.
const isDrift = computed(
  () =>
    props.session.drift_runs.length > 0 ||
    (props.session.best_drift ?? 0) > 0 ||
    props.session.segments.some((s) => s.kind === "drift"),
);
// One row per drift score, high→low so the headline run leads (rank #1 = best).
const driftRows = computed(() => [...props.session.drift_runs].sort((a, b) => b.score - a.score));

// Clips ride inline on their score row; the captures gallery holds the rest
// (screenshots, manual clips) so nothing appears twice.
const tiedClipIds = computed(() => new Set(props.session.drift_runs.map((r) => r.clip?.id).filter(Boolean)));
const captures = computed(() => props.session.media.filter((m) => !tiedClipIds.value.has(m.id)));

// One lightbox for the whole card: plays a clip, or shows a still.
const active = ref<MediaItem | null>(null);
function openMedia(m: MediaItem | null | undefined) {
  if (m && isRealMedia(m)) active.value = m;
}

const durationLabel = computed(() => {
  const end = props.session.left_at ?? Date.now();
  const mins = Math.max(0, Math.round((end - props.session.joined_at) / 60000));
  if (mins < 60) return `${mins}m`;
  const h = Math.floor(mins / 60);
  return `${h}h ${mins % 60}m`;
});

const dateLabel = computed(() => fmtDate(props.session.joined_at));

function lapDelta(ms: number): string {
  const best = props.session.best_lap_ms ?? 0;
  if (!best || ms === best) return "";
  return `+${((ms - best) / 1000).toFixed(3)}`;
}

function isRealMedia(m: MediaItem): boolean {
  return !!m.url && m.url !== "#";
}

function segMetric(r: DriverResult) {
  return describeResult(r);
}

function submitTag() {
  const t = newTag.value.trim();
  if (!t) return;
  if (props.session.tags.some((x) => x.toLowerCase() === t.toLowerCase())) {
    newTag.value = "";
    return;
  }
  emit("add-tag", t);
  newTag.value = "";
}
</script>

<template>
  <section :id="`session-${session.id}`" class="profile-drive" :class="{ 'is-live': isLive }">
    <button type="button" class="profile-drive-summary" :aria-label="`${expanded ? 'Collapse' : 'Expand'} session at ${session.track.name}`" :aria-expanded="expanded" :aria-controls="`session-details-${session.id}`" @click="expanded = !expanded">
      <TrackImage :track-key="session.track.key" :config="session.track.config" :overlay="false" class="profile-drive-photo" />
      <span class="profile-drive-identity">
        <span class="profile-drive-date">{{ dateLabel }} · {{ durationLabel }} <span v-if="isLive" class="profile-status">On track</span></span>
        <span class="profile-drive-title">{{ session.track.name }}</span>
        <span class="profile-drive-car">{{ session.car.name }}</span>
        <span v-if="session.track.country || sessionTrackVersion" class="profile-drive-place">{{ [session.track.country, sessionTrackVersion ? `Version ${sessionTrackVersion}` : ''].filter(Boolean).join(' · ') }}</span>
      </span>
      <span class="profile-drive-metric">
        <template v-if="session.best_drift"><b>{{ fmtScore(session.best_drift) }}</b><span>Best drift</span></template>
        <template v-else-if="session.best_lap_ms"><b>{{ lapTime(session.best_lap_ms) }}</b><span>Best lap</span></template>
        <span>{{ session.laps_total }} {{ session.laps_total === 1 ? 'lap' : 'laps' }}</span>
      </span>
      <span class="profile-drive-open">{{ expanded ? 'Close' : 'Details' }}<Icon :name="expanded ? 'arrowUp' : 'arrowDown'" :size="17" /></span>
    </button>

    <div v-if="expanded" :id="`session-details-${session.id}`" class="profile-drive-body">
      <div v-if="session.tags.length || taggable" class="profile-drive-tags">
        <span v-for="t in session.tags" :key="t" class="profile-tag">
          {{ t }}
          <button v-if="taggable" type="button" :aria-label="`Remove tag ${t}`" @click="emit('remove-tag', t)"><Icon name="x" :size="14" /></button>
        </span>
        <form v-if="taggable" class="profile-tag-form" @submit.prevent="submitTag">
          <input v-model="newTag" type="text" maxlength="40" placeholder="Give this drive a tag…" aria-label="Add session tag" />
          <Button variant="ghost" size="sm" type="submit">Add tag</Button>
        </form>
      </div>
      <p v-if="session.best_drift && session.best_lap_ms" class="profile-drive-note">Fastest lap · {{ lapTime(session.best_lap_ms) }}</p>

      <section v-if="!isDrift && session.segments.length" class="profile-drive-section">
        <h3>Sessions on track</h3>
        <ul>
          <li v-for="(r, i) in session.segments" :key="r.session_id + '-' + i" class="profile-result-row">
            <div><b>{{ sessionKindLabel[r.kind] }}</b><p>{{ segMetric(r).secondary }}</p></div>
            <div class="profile-drive-metric"><b :class="segMetric(r).tone === 'warn' ? 'text-warn' : segMetric(r).tone === 'accent' ? 'text-accent' : ''">{{ segMetric(r).primary }}</b><span>{{ segMetric(r).unit }}</span></div>
          </li>
        </ul>
      </section>
      <section v-if="!isDrift && session.laps.length" class="profile-drive-section">
        <h3>Lap times <span>{{ session.laps.length }} laps</span></h3>
        <div class="profile-lap-table">
          <table>
            <thead><tr><th>Lap</th><th>Time</th><th>Δ Best</th><th>Cuts</th></tr></thead>
            <tbody><tr v-for="l in session.laps" :key="l.lap" :class="{ 'is-best': l.is_best }">
              <td>{{ l.lap }}</td><td>{{ lapTime(l.laptime_ms) }} <span v-if="l.is_best" class="profile-best">Best</span></td><td>{{ lapDelta(l.laptime_ms) || '—' }}</td><td :class="l.cuts ? 'text-warn' : ''">{{ l.cuts || '—' }}</td>
            </tr></tbody>
          </table>
        </div>
      </section>
      <section v-if="isDrift && driftRows.length" class="profile-drive-section">
        <h3>Drift runs <span>{{ driftRows.length }} runs</span></h3>
        <ol>
          <li v-for="(run, i) in driftRows" :key="run.id" class="profile-result-row">
            <span class="profile-run-rank">{{ i + 1 }}</span>
            <div class="min-w-0 flex-1"><b class="text-accent">{{ fmtScore(run.score) }}</b> <span class="text-muted">points</span><span v-if="i === 0" class="profile-best">Best</span><p>{{ timeAgo(run.ended_at) }}</p></div>
            <button v-if="run.clip" type="button" class="profile-run-clip" :disabled="!isRealMedia(run.clip)" :aria-label="isRealMedia(run.clip) ? 'Play drift clip' : 'Clip unavailable'" @click="openMedia(run.clip)">
              <img v-if="run.clip.thumb_url" :src="run.clip.thumb_url" alt="" loading="lazy" />
              <video v-else-if="isRealMedia(run.clip)" :src="run.clip.url" preload="metadata" muted playsinline />
              <span><Icon name="play" :size="20" />{{ run.clip.duration_s ? fmtClipDuration(run.clip.duration_s) : 'Play' }}</span>
            </button>
          </li>
        </ol>
      </section>
      <section v-if="captures.length" class="profile-drive-section">
        <h3>Saved moments <span>{{ captures.length }} captures</span></h3>
        <div class="profile-captures">
          <article v-for="m in captures" :key="m.id">
            <button type="button" class="profile-capture-preview" :disabled="!isRealMedia(m)" :aria-label="`${m.kind === 'clip' ? 'Play' : 'View'} ${m.caption || (m.kind === 'clip' ? 'clip' : 'screenshot')}`" @click="openMedia(m)">
              <img v-if="m.kind === 'screenshot' && isRealMedia(m)" :src="m.url" alt="" loading="lazy" />
              <img v-else-if="m.thumb_url" :src="m.thumb_url" alt="" loading="lazy" />
              <video v-else-if="isRealMedia(m)" :src="m.url" preload="metadata" muted playsinline />
              <Icon v-else :name="m.kind === 'clip' ? 'play' : 'camera'" :size="28" />
              <span v-if="m.kind === 'clip'" class="profile-capture-play"><Icon name="play" :size="22" />{{ m.duration_s ? fmtClipDuration(m.duration_s) : 'Play clip' }}</span>
            </button>
            <div class="profile-capture-caption">
              <div><b>{{ m.caption || (m.kind === 'clip' ? 'Clip' : 'Screenshot') }}</b><p>{{ timeAgo(m.captured_at) }}<span v-if="m.trigger"> · +{{ fmtScore(m.trigger.delta) }} points</span></p></div>
              <MediaActions v-if="isRealMedia(m)" :item="m" :can-delete="canOperate" :deleting="deletingIds?.has(m.id) ?? false" @download="emit('download-media', m)" @delete="emit('delete-media', m)" />
            </div>
          </article>
        </div>
      </section>
      <p v-if="!session.segments.length && !session.laps.length && !session.drift_runs.length && !session.media.length" class="profile-drive-note">No laps, drift runs or highlights recorded in this session.</p>
      <div v-if="assignable" class="profile-drive-actions">
        <Button variant="ghost" size="sm" @click="assignOpen = true"><Icon name="user" :size="16" /> Reassign driver</Button>
        <Button v-if="!isLive" variant="danger" size="sm" @click="emit('delete-session')"><Icon name="trash" :size="16" /> Delete session</Button>
      </div>
    </div>

    <!-- Lightbox: plays the clicked clip / shows the clicked still -->
    <Modal :open="!!active" wide :title="active?.caption || (active?.kind === 'clip' ? 'Clip' : 'Capture')" @close="active = null">
      <video
        v-if="active?.kind === 'clip'"
        :src="active.url"
        class="mx-auto max-h-[78vh] w-full rounded-md bg-black object-contain"
        controls
        autoplay
        playsinline
      />
      <img v-else-if="active" :src="active.url" :alt="active.caption" class="mx-auto max-h-[78vh] w-full rounded-md object-contain" />
      <template #footer>
        <!-- iOS video player has no download; give it an explicit button -->
        <Button v-if="active && isRealMedia(active)" variant="ghost" @click="emit('download-media', active)">
          <Icon name="download" :size="14" class="mr-1.5" /> Download
        </Button>
        <Button @click="active = null">Close</Button>
      </template>
    </Modal>

    <!-- Reassign-driver modal -->
    <Modal :open="assignOpen" title="Assign session to driver" @close="assignOpen = false">
      <p class="mb-3 text-xs text-muted">
        Attribute every lap, drift run and clip in this session to a driver. Pick the account's own name to revert.
      </p>
      <ul class="space-y-1">
        <li>
          <button
            type="button"
            class="flex w-full items-center gap-2 rounded-md border px-3 py-2 text-left text-sm transition-colors"
            :class="!session.guest_driver_id ? 'border-accent/50 bg-accent-dim font-semibold text-accent' : 'border-line text-text hover:border-line-hi hover:bg-surface-2'"
            @click="pick(null)"
          >
            <Icon name="user" :size="14" /> Account driver (own name)
          </button>
        </li>
        <li v-for="g in guests" :key="g.id">
          <button
            type="button"
            class="flex w-full items-center gap-2 rounded-md border px-3 py-2 text-left text-sm transition-colors"
            :class="session.guest_driver_id === g.id ? 'border-accent/50 bg-accent-dim font-semibold text-accent' : 'border-line text-text hover:border-line-hi hover:bg-surface-2'"
            @click="pick(g.id)"
          >
            {{ g.name }}
          </button>
        </li>
        <li v-if="!guests || !guests.length" class="px-3 py-2 text-xs text-dim">
          No guests yet — add them in the Guest roster.
        </li>
      </ul>
    </Modal>
  </section>
</template>
