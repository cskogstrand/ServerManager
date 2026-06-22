<script setup lang="ts">
// One driver "session" (connection: connect→disconnect) as a collapsible card.
// Header shows the stint at a glance (track, car, time range, KPIs, tags); the
// body groups the per-AC-session segments, per-lap times, drift runs + clips and
// captured media that all happened during that one connection.
import { computed, ref } from "vue";
import { lapTime } from "@/lib/raceTelemetry";
import { fmtScore, fmtDate, timeAgo, sessionKindLabel, describeResult } from "@/lib/driversApi";
import { fmtClipDuration } from "@/lib/useDriverCapture";
import type { DriverSession, DriverResult, MediaItem } from "@/types/driverStats";
import type { GuestDriver } from "@/lib/guestDriversApi";
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
}>();

const expanded = ref(props.defaultOpen ?? false);
const newTag = ref("");

// A real, taggable session (id "0" is the synthetic "ungrouped" legacy bucket).
const taggable = computed(() => props.canOperate && props.session.id !== "0");
const isLive = computed(() => props.session.left_at === null);

// Guest attribution: who this whole session counts for. A roster guest, or the
// GUID's own name when unassigned. Resolved against the list the parent loads.
// Reassigning is infrequent, so it lives behind a button + modal in the body.
const assignable = computed(() => props.canOperate && props.session.id !== "0");
const guestName = computed(() => {
  const id = props.session.guest_driver_id;
  if (!id) return "";
  return props.guests?.find((g) => g.id === id)?.name ?? "Guest driver";
});
const assignOpen = ref(false);
function pick(guestDriverId: number | null) {
  assignOpen.value = false;
  emit("assign", guestDriverId);
}

const clips = computed(() => props.session.media.filter((m) => m.kind === "clip"));
const screenshots = computed(() => props.session.media.filter((m) => m.kind === "screenshot"));

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
  <section
    :id="`session-${session.id}`"
    class="reveal overflow-hidden rounded-lg border bg-surface-2/30 transition-colors"
    :class="isLive ? 'border-ok/40' : 'border-line'"
  >
    <!-- HEADER -->
    <header
      class="flex cursor-pointer flex-col gap-3 p-3.5 sm:p-4"
      role="button"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      <div class="flex items-start gap-3">
        <button
          type="button"
          class="mt-0.5 grid size-6 shrink-0 place-items-center rounded-md border border-line bg-surface-3 text-muted transition-transform"
          :class="expanded ? 'rotate-180' : ''"
          tabindex="-1"
        >
          <Icon name="arrowDown" :size="14" />
        </button>

        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="truncate text-sm font-bold tracking-tight text-text">{{ session.track.name }}</h3>
            <span
              v-if="isLive"
              class="inline-flex items-center gap-1.5 rounded-full border border-ok/40 bg-ok-glow px-2 py-0.5 text-[10px] font-bold tracking-wide text-ok uppercase"
            >
              <span class="size-1.5 rounded-full bg-ok live-dot" /> Live
            </span>
            <span v-if="session.track.country" class="text-[11px] text-dim">{{ session.track.country }}</span>
          </div>
          <div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted">
            <span class="inline-flex items-center gap-1"><Icon name="car" :size="12" class="text-dim" /> {{ session.car.name }}</span>
            <span class="inline-flex items-center gap-1"><Icon name="calendar" :size="12" class="text-dim" /> {{ dateLabel }}</span>
            <span class="inline-flex items-center gap-1"><Icon name="clock" :size="12" class="text-dim" /> {{ durationLabel }}</span>
          </div>
        </div>

        <!-- KPI strip -->
        <div class="flex shrink-0 items-center gap-3 text-right">
          <div v-if="session.best_drift" class="hidden sm:block">
            <div class="font-mono text-sm font-bold tabular-nums text-accent">{{ fmtScore(session.best_drift) }}</div>
            <div class="text-[9px] font-bold tracking-wide text-dim uppercase">Best drift</div>
          </div>
          <div v-if="session.best_lap_ms" class="hidden sm:block">
            <div class="font-mono text-sm font-bold tabular-nums text-text">{{ lapTime(session.best_lap_ms) }}</div>
            <div class="text-[9px] font-bold tracking-wide text-dim uppercase">Best lap</div>
          </div>
          <div>
            <div class="font-mono text-sm font-bold tabular-nums text-text">{{ session.laps_total }}</div>
            <div class="text-[9px] font-bold tracking-wide text-dim uppercase">Laps</div>
          </div>
        </div>
      </div>

      <!-- TAGS -->
      <div class="flex flex-wrap items-center gap-1.5" @click.stop>
        <span
          v-for="t in session.tags"
          :key="t"
          class="group inline-flex items-center gap-1 rounded-full border border-accent/35 bg-accent-dim px-2 py-0.5 text-[11px] font-semibold text-accent"
        >
          {{ t }}
          <button
            v-if="taggable"
            type="button"
            class="grid size-3.5 cursor-pointer place-items-center rounded-full text-accent/70 hover:bg-accent/20 hover:text-accent"
            title="Remove tag"
            @click="emit('remove-tag', t)"
          >
            <Icon name="x" :size="10" />
          </button>
        </span>

        <span v-if="!session.tags.length && !taggable" class="text-[11px] text-dim">No tags</span>

        <!-- Add tag inline -->
        <div v-if="taggable" class="inline-flex items-center">
          <input
            v-model="newTag"
            type="text"
            maxlength="40"
            placeholder="+ tag"
            class="h-6 w-20 rounded-full border border-dashed border-line bg-surface px-2.5 text-[11px] text-text placeholder:text-dim focus:w-28 focus:border-accent/50 focus:outline-none"
            @keydown.enter.prevent="submitTag"
            @keydown.stop
          />
        </div>
      </div>
    </header>

    <!-- BODY -->
    <div v-if="expanded" class="border-t border-line/70 px-3.5 pb-4 pt-3 sm:px-4">
      <!-- Attribution: who this session's results count for -->
      <div v-if="assignable || guestName" class="mb-4 flex items-center gap-2 rounded-md border border-line/60 bg-surface/40 px-3 py-2">
        <Icon name="user" :size="14" class="text-dim" />
        <span class="text-xs text-muted">Driven by</span>
        <span class="text-sm font-semibold" :class="guestName ? 'text-accent' : 'text-text'">{{ guestName || "Account driver" }}</span>
        <Button v-if="assignable" variant="ghost" size="sm" class="ml-auto" @click="assignOpen = true">Reassign</Button>
      </div>

      <!-- Segments -->
      <div v-if="session.segments.length" class="mb-4">
        <div class="mb-1.5 text-[11px] font-bold tracking-wide text-dim uppercase">Sessions on track</div>
        <ul class="space-y-1">
          <li
            v-for="(r, i) in session.segments"
            :key="r.session_id + '-' + i"
            class="flex items-center gap-3 rounded-md border border-line/60 bg-surface/40 px-2.5 py-1.5"
          >
            <span
              class="grid w-16 shrink-0 place-items-center rounded border py-0.5 text-[10px] font-bold tracking-wide uppercase"
              :class="r.kind === 'drift' ? 'border-accent/40 bg-accent-dim text-accent' : r.position === 1 ? 'border-warn/45 bg-warn-glow text-warn' : 'border-line bg-surface-2 text-muted'"
            >
              {{ sessionKindLabel[r.kind] }}
            </span>
            <div class="min-w-0 flex-1 text-xs text-muted truncate">{{ segMetric(r).secondary }}</div>
            <div class="shrink-0 text-right">
              <span
                class="font-mono text-xs font-bold tabular-nums"
                :class="segMetric(r).tone === 'warn' ? 'text-warn' : segMetric(r).tone === 'accent' ? 'text-accent' : 'text-text'"
              >{{ segMetric(r).primary }}</span>
              <span class="ml-1 text-[9px] font-bold tracking-wide text-dim uppercase">{{ segMetric(r).unit }}</span>
            </div>
          </li>
        </ul>
      </div>

      <!-- Laps -->
      <div v-if="session.laps.length" class="mb-4">
        <div class="mb-1.5 flex items-center gap-2 text-[11px] font-bold tracking-wide text-dim uppercase">
          <Icon name="gauge" :size="13" /> Lap times <span class="text-dim/70">({{ session.laps.length }})</span>
        </div>
        <div class="overflow-hidden rounded-md border border-line/60">
          <table class="w-full text-xs">
            <thead>
              <tr class="bg-surface-3/60 text-left text-[10px] font-bold tracking-wide text-dim uppercase">
                <th class="px-3 py-1.5 font-bold">Lap</th>
                <th class="px-3 py-1.5 font-bold">Time</th>
                <th class="px-3 py-1.5 text-right font-bold">Δ Best</th>
                <th class="px-3 py-1.5 text-right font-bold">Cuts</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="l in session.laps"
                :key="l.lap"
                class="border-t border-line/40"
                :class="l.is_best ? 'bg-accent-dim/40' : ''"
              >
                <td class="px-3 py-1.5 font-mono tabular-nums text-muted">{{ l.lap }}</td>
                <td class="px-3 py-1.5 font-mono font-semibold tabular-nums" :class="l.is_best ? 'text-accent' : 'text-text'">
                  {{ lapTime(l.laptime_ms) }}
                  <span v-if="l.is_best" class="ml-1 text-[9px] font-bold tracking-wide uppercase">best</span>
                </td>
                <td class="px-3 py-1.5 text-right font-mono tabular-nums text-dim">{{ lapDelta(l.laptime_ms) || "—" }}</td>
                <td class="px-3 py-1.5 text-right font-mono tabular-nums" :class="l.cuts ? 'text-warn' : 'text-dim'">{{ l.cuts || "—" }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Drift runs -->
      <div v-if="session.drift_runs.length" class="mb-4">
        <div class="mb-1.5 flex items-center gap-2 text-[11px] font-bold tracking-wide text-dim uppercase">
          <Icon name="activity" :size="13" /> Drift runs <span class="text-dim/70">({{ session.drift_runs.length }})</span>
        </div>
        <div class="grid gap-2 sm:grid-cols-2">
          <div
            v-for="run in session.drift_runs"
            :key="run.id"
            class="overflow-hidden rounded-md border border-line/60 bg-surface/40"
          >
            <div
              v-if="run.clip && isRealMedia(run.clip)"
              class="aspect-video w-full overflow-hidden border-b border-line/60"
            >
              <video :src="run.clip.url" class="size-full bg-black object-cover" preload="none" controls playsinline />
            </div>
            <div class="flex items-center justify-between gap-2 px-2.5 py-1.5">
              <div class="flex items-center gap-2">
                <Icon name="activity" :size="13" class="text-accent" />
                <span class="font-mono text-sm font-bold tabular-nums text-accent">{{ fmtScore(run.score) }}</span>
                <span class="text-[10px] font-bold tracking-wide text-dim uppercase">pts</span>
              </div>
              <span class="text-[10px] text-dim">{{ timeAgo(run.ended_at) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Media (manual / screenshots not tied to a run) -->
      <div v-if="screenshots.length || clips.length">
        <div class="mb-1.5 flex items-center gap-2 text-[11px] font-bold tracking-wide text-dim uppercase">
          <Icon name="film" :size="13" /> Highlights <span class="text-dim/70">({{ session.media.length }})</span>
        </div>
        <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          <article
            v-for="m in session.media"
            :key="m.id"
            class="group overflow-hidden rounded-md border border-line/60 bg-surface/40"
          >
            <div class="relative aspect-video overflow-hidden bg-black">
              <video v-if="m.kind === 'clip' && isRealMedia(m)" :src="m.url" class="size-full object-cover" preload="none" controls playsinline />
              <img v-else-if="isRealMedia(m)" :src="m.url" alt="" loading="lazy" class="size-full object-cover" />
              <div v-else class="grid size-full place-items-center text-text/25">
                <Icon :name="m.kind === 'clip' ? 'play' : 'camera'" :size="20" />
              </div>
              <span
                v-if="m.trigger"
                class="pointer-events-none absolute top-1.5 left-1.5 inline-flex items-center gap-1 rounded border border-warn/45 bg-warn-glow px-1.5 py-0.5 font-mono text-[10px] font-bold text-warn"
              >
                <Icon name="arrowUp" :size="10" /> +{{ fmtScore(m.trigger.delta) }}
              </span>
              <span v-if="m.duration_s" class="pointer-events-none absolute top-1.5 right-1.5 rounded bg-bg/60 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-text/90">
                {{ fmtClipDuration(m.duration_s) }}
              </span>
            </div>
            <div class="flex items-center gap-2 px-2 py-1.5">
              <div class="min-w-0 flex-1">
                <div class="truncate text-[11px] font-semibold text-text">{{ m.caption }}</div>
                <div class="text-[10px] text-muted">{{ timeAgo(m.captured_at) }}</div>
              </div>
              <MediaActions
                v-if="isRealMedia(m)"
                :item="m"
                :can-delete="canOperate"
                :deleting="deletingIds?.has(m.id) ?? false"
                @download="emit('download-media', m)"
                @delete="emit('delete-media', m)"
              />
            </div>
          </article>
        </div>
      </div>

      <!-- Empty body -->
      <p
        v-if="!session.segments.length && !session.laps.length && !session.drift_runs.length && !session.media.length"
        class="py-3 text-center text-xs text-muted"
      >
        No laps, drift runs or highlights recorded in this session.
      </p>
    </div>

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
          No guest drivers yet — add them on the Guest Drivers page.
        </li>
      </ul>
    </Modal>
  </section>
</template>

<style scoped>
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
