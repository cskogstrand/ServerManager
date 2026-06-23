<script setup lang="ts">
// All-servers leaderboard, in the broadcast graphics style. Lists EVERY scored
// result — one row per drift run and per timed-lap session, not one per driver
// — and lets you filter by discipline (drift / lap), track, car and recency.
// Self-contained: loads its own feed on mount and refreshes on a slow timer so
// it can stand as a permanent trackside display.
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { listScores, fmtScore, timeAgo } from "@/lib/driversApi";
import { lapTime } from "@/lib/raceTelemetry";
import type { MediaItem, ScoreEntry } from "@/types/driverStats";
import { ApiError } from "@/lib/api";
import { type GuestDriver, listGuestDrivers, assignScore } from "@/lib/guestDriversApi";
import { useAuthStore } from "@/stores/auth";
import { useToastStore } from "@/stores/toast";
import Icon from "@/components/ui/Icon.vue";
import Modal from "@/components/ui/Modal.vue";
import UiSelect from "@/components/ui/Select.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";

const auth = useAuthStore();
const toast = useToastStore();
const route = useRoute();
const router = useRouter();

const scores = ref<ScoreEntry[]>([]);
const loaded = ref(false);

async function load() {
  try {
    scores.value = await listScores();
  } catch {
    scores.value = [];
  } finally {
    loaded.value = true;
  }
}

// --- filters ----------------------------------------------------------------
// State seeds from the URL query so a filtered board is shareable/bookmarkable;
// changes write back with router.replace (no history entry — see the watch below).
type Discipline = "drift" | "lap";
type Period = "all" | "6h" | "today" | "1d" | "7d" | "30d";
const periodOptions: { value: Period; label: string }[] = [
  { value: "all", label: "All time" },
  { value: "6h", label: "6h" },
  { value: "today", label: "Today" },
  { value: "1d", label: "24h" },
  { value: "7d", label: "7d" },
  { value: "30d", label: "30d" },
];

const qstr = (v: unknown) => (typeof v === "string" ? v : "");
const discipline = ref<Discipline>(route.query.d === "lap" ? "lap" : "drift");
const trackFilter = ref(qstr(route.query.track)); // "" = all; else `${key} ${config}`
const carFilter = ref(qstr(route.query.car)); // "" = all; else car key
const search = ref(qstr(route.query.q));
const period = ref<Period>(periodOptions.some((o) => o.value === route.query.period) ? (route.query.period as Period) : "all");

// --- pagination -------------------------------------------------------------
const PER_PAGE = 25;
const page = ref(1);

// Oldest epoch-ms timestamp still included for a period, or -Infinity for "all".
// "today" is the local calendar day (since midnight) — distinct from the rolling
// "24h" window, which is a fixed duration back from now.
function periodCutoff(p: Period, now: number): number {
  switch (p) {
    case "all":
      return -Infinity;
    case "6h":
      return now - 6 * 3_600_000;
    case "1d":
      return now - 86_400_000;
    case "7d":
      return now - 7 * 86_400_000;
    case "30d":
      return now - 30 * 86_400_000;
    case "today": {
      const d = new Date(now);
      d.setHours(0, 0, 0, 0);
      return d.getTime();
    }
  }
}

function trackId(s: ScoreEntry): string {
  return `${s.track.key} ${s.track.config ?? ""}`;
}

// Filter options are built from the full feed so they stay stable when the
// discipline toggles (a track with only drift runs still lists under Lap, it
// just yields an empty board — expected).
const trackOptions = computed(() => {
  const m = new Map<string, string>();
  for (const s of scores.value) {
    const label = s.track.config ? `${s.track.name} · ${s.track.config}` : s.track.name;
    if (!m.has(trackId(s))) m.set(trackId(s), label);
  }
  return [...m].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label));
});
const carOptions = computed(() => {
  const m = new Map<string, string>();
  for (const s of scores.value) if (!m.has(s.car.key)) m.set(s.car.key, s.car.name);
  return [...m].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label));
});

const filtered = computed<ScoreEntry[]>(() => {
  const q = search.value.trim().toLowerCase();
  const minDate = periodCutoff(period.value, Date.now());
  const list = scores.value.filter((s) => {
    if (s.kind !== discipline.value) return false;
    if (trackFilter.value && trackId(s) !== trackFilter.value) return false;
    if (carFilter.value && s.car.key !== carFilter.value) return false;
    if (s.date < minDate) return false;
    return !q || s.driver.toLowerCase().includes(q);
  });
  list.sort((a, b) =>
    discipline.value === "drift"
      ? (b.drift_score ?? 0) - (a.drift_score ?? 0)
      : (a.best_lap_ms ?? 0) - (b.best_lap_ms ?? 0),
  );
  return list;
});

const pageCount = computed(() => Math.max(1, Math.ceil(filtered.value.length / PER_PAGE)));
const paged = computed(() => filtered.value.slice((page.value - 1) * PER_PAGE, page.value * PER_PAGE));

const hasFilters = computed(
  () => !!trackFilter.value || !!carFilter.value || !!search.value.trim() || period.value !== "all",
);

// Sync filters → URL (replace, so no history spam) and snap back to page 1.
watch([discipline, trackFilter, carFilter, search, period], () => {
  page.value = 1;
  const q: Record<string, string> = {};
  if (discipline.value !== "drift") q.d = discipline.value;
  if (trackFilter.value) q.track = trackFilter.value;
  if (carFilter.value) q.car = carFilter.value;
  if (search.value.trim()) q.q = search.value.trim();
  if (period.value !== "all") q.period = period.value;
  void router.replace({ query: q }).catch(() => {});
});

// Keep the page in range when the result set shrinks (filter change, reload).
watch(pageCount, (n) => {
  if (page.value > n) page.value = n;
});

// --- Highlight-clip popup ---------------------------------------------------
// A drift row whose run was auto-captured opens the clip in a modal with a
// download button. Real captures resolve to a served file; mock items ("#")
// show a placeholder.
const clip = ref<MediaItem | null>(null);
function isRealMedia(m: MediaItem | null | undefined): boolean {
  return !!m?.url && m.url !== "#";
}
function downloadClip(m: MediaItem | null) {
  if (!isRealMedia(m)) return;
  const href = m!.url + (m!.url.includes("?") ? "&" : "?") + "download=1";
  const a = document.createElement("a");
  a.href = href;
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  a.remove();
}
function clearFilters() {
  trackFilter.value = "";
  carFilter.value = "";
  search.value = "";
  period.value = "all";
}

// --- Reassign driver (guest-driver attribution) -----------------------------
// Operators can set which person a result belongs to — handy when several people
// share one account. The roster is fetched lazily the first time the editor opens.
const roster = ref<GuestDriver[]>([]);
const editing = ref<ScoreEntry | null>(null);
const editSel = ref<number>(0);
const savingEdit = ref(false);

const extraOptions = computed(() => [
  { value: 0, label: "— Account's own name —" },
  ...roster.value.map((d) => ({ value: d.id, label: d.name })),
]);

async function openEdit(s: ScoreEntry) {
  editing.value = s;
  editSel.value = s.guest_driver_id ?? 0;
  if (!roster.value.length) {
    try {
      roster.value = await listGuestDrivers();
    } catch {
      /* leave empty — the editor shows a hint to add some */
    }
  }
}

async function saveEdit() {
  if (!editing.value) return;
  savingEdit.value = true;
  try {
    await assignScore(editing.value.id, editSel.value || null);
    toast.success("Driver updated.");
    editing.value = null;
    await load();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    savingEdit.value = false;
  }
}

// Slow refresh keeps a standing display current without heavy polling.
let poll: ReturnType<typeof setInterval> | null = null;
onMounted(() => {
  void load();
  poll = setInterval(() => void load(), 30000);
});
onBeforeUnmount(() => {
  if (poll) clearInterval(poll);
});
</script>

<template>
  <div class="flex h-full w-full flex-col gap-3">
    <!-- ░░ Filter bar ░░ -->
    <div class="flex flex-wrap items-center gap-2">
      <!-- Discipline -->
      <div class="flex rounded-md border border-line bg-surface-2/70 p-0.5">
        <button
          type="button"
          class="h-8 cursor-pointer rounded-sm px-3 text-xs font-bold tracking-wide uppercase transition-colors"
          :class="discipline === 'drift' ? 'bg-accent-dim text-accent' : 'text-muted hover:text-text'"
          @click="discipline = 'drift'"
        >
          Drift
        </button>
        <button
          type="button"
          class="h-8 cursor-pointer rounded-sm px-3 text-xs font-bold tracking-wide uppercase transition-colors"
          :class="discipline === 'lap' ? 'bg-accent-dim text-accent' : 'text-muted hover:text-text'"
          @click="discipline = 'lap'"
        >
          Lap
        </button>
      </div>

      <!-- Track -->
      <select
        v-model="trackFilter"
        class="h-9 max-w-[44vw] rounded-md border border-line bg-surface-2/70 px-2 text-sm text-text transition-colors focus:border-accent/60 focus:outline-none"
        title="Filter by track"
      >
        <option value="">All tracks</option>
        <option v-for="o in trackOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
      </select>

      <!-- Car -->
      <select
        v-model="carFilter"
        class="h-9 max-w-[44vw] rounded-md border border-line bg-surface-2/70 px-2 text-sm text-text transition-colors focus:border-accent/60 focus:outline-none"
        title="Filter by car"
      >
        <option value="">All cars</option>
        <option v-for="o in carOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
      </select>

      <!-- Period -->
      <div class="flex rounded-md border border-line bg-surface-2/70 p-0.5">
        <button
          v-for="o in periodOptions"
          :key="o.value"
          type="button"
          class="h-8 cursor-pointer rounded-sm px-2.5 text-xs font-semibold transition-colors"
          :class="period === o.value ? 'bg-accent-dim text-accent' : 'text-muted hover:text-text'"
          @click="period = o.value"
        >
          {{ o.label }}
        </button>
      </div>

      <!-- Search -->
      <div class="relative min-w-37.5 flex-1">
        <Icon name="search" :size="15" class="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-dim" />
        <input
          v-model="search"
          type="search"
          placeholder="Driver…"
          class="h-9 w-full rounded-md border border-line bg-surface-2/70 pr-2 pl-8 text-sm text-text transition-colors placeholder:text-dim focus:border-accent/60 focus:outline-none"
        />
      </div>

      <button
        v-if="hasFilters"
        type="button"
        class="inline-flex h-9 cursor-pointer items-center gap-1.5 rounded-md border border-line bg-surface-2/70 px-2.5 text-xs font-semibold text-muted transition-colors hover:border-danger/50 hover:text-danger"
        title="Clear filters"
        @click="clearFilters"
      >
        <Icon name="x" :size="14" />
        Clear
      </button>
    </div>

    <!-- ░░ Board ░░ -->
    <section class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-line bg-surface/30 backdrop-blur-sm">
      <header class="flex items-center gap-2 border-b border-line/60 px-4 py-3">
        <Icon :name="discipline === 'drift' ? 'broadcast' : 'activity'" :size="16" :class="discipline === 'drift' ? 'text-accent' : 'text-ok'" />
        <h2 class="numerals text-lg tracking-tight">{{ discipline === "drift" ? "Top Drift Runs" : "Best Laps" }}</h2>
        <span class="ml-auto font-mono text-[11px] tracking-wider text-dim uppercase">
          {{ filtered.length }} · all servers
        </span>
      </header>

      <div class="min-h-0 flex-1 overflow-auto">
        <table class="w-full border-collapse text-sm">
          <thead class="sticky top-0 z-10 bg-surface/85 backdrop-blur-sm">
            <tr class="border-b border-line/60 text-left font-mono text-[10px] tracking-wider text-dim uppercase">
              <th class="w-12 px-3 py-2 text-center">#</th>
              <th class="w-full px-3 py-2">Driver</th>
              <th class="hidden px-3 py-2 whitespace-nowrap sm:table-cell">When</th>
              <th class="px-3 py-2 text-right whitespace-nowrap">{{ discipline === "drift" ? "Drift" : "Lap" }}</th>
              <th class="px-3 py-2 text-right whitespace-nowrap">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(s, i) in paged"
              :key="s.id"
              class="border-b border-line/40"
              :class="(page - 1) * PER_PAGE + i === 0 ? 'bg-accent-glow/5' : i % 2 ? 'bg-surface/30' : ''"
            >
              <td class="px-3 py-2 text-center">
                <span
                  class="numerals inline-grid size-8 place-items-center rounded-lg text-base font-bold tabular-nums"
                  :class="(page - 1) * PER_PAGE + i === 0 ? (discipline === 'drift' ? 'bg-accent text-bg' : 'bg-ok text-bg') : 'bg-surface-3 text-muted'"
                >{{ (page - 1) * PER_PAGE + i + 1 }}</span>
              </td>

              <td class="min-w-0 px-3 py-2">
                <div class="flex items-center gap-1.5">
                  <span v-if="s.online" class="size-1.5 shrink-0 rounded-full bg-ok live-dot" title="On track now" />
                  <span class="truncate text-sm font-bold">{{ s.driver }}</span>
                </div>
                <div class="truncate font-mono text-[11px] text-dim">
                  {{ s.car.name }}<span class="text-dim/60"> · </span>{{ s.track.name }}
                </div>
              </td>

              <td class="hidden px-3 py-2 font-mono text-[11px] whitespace-nowrap text-dim sm:table-cell">
                {{ timeAgo(s.date) }}
              </td>

              <td class="px-3 py-2 text-right whitespace-nowrap">
                <span
                  class="numerals text-xl font-semibold tabular-nums"
                  :class="discipline === 'drift' ? 'text-accent' : 'text-ok'"
                >
                  {{ discipline === "drift" ? fmtScore(s.drift_score ?? 0) : lapTime(s.best_lap_ms ?? 0) }}
                </span>
                <span v-if="discipline === 'drift'" class="ml-1 font-mono text-[10px] tracking-wider text-dim uppercase">pts</span>
              </td>

              <td class="px-3 py-2">
                <div class="flex items-center justify-end gap-1.5">
                  <button
                    v-if="discipline === 'drift' && s.clip"
                    type="button"
                    class="grid size-8 shrink-0 place-items-center rounded-lg border border-line bg-surface-2/70 text-muted transition-colors hover:border-accent/60 hover:text-accent"
                    title="Watch the highlight clip from this run"
                    @click="clip = s.clip ?? null"
                  >
                    <Icon name="film" :size="15" />
                  </button>

                  <button
                    v-if="auth.canOperate"
                    type="button"
                    title="Set which driver this result belongs to"
                    class="grid size-8 shrink-0 place-items-center rounded-lg border bg-surface-2/70 transition-colors"
                    :class="s.guest_driver_id
                      ? 'border-accent/50 text-accent hover:border-accent'
                      : 'border-line text-muted hover:border-accent/60 hover:text-accent'"
                    @click="openEdit(s)"
                  >
                    <Icon name="edit" :size="14" />
                  </button>

                  <RouterLink
                    :to="s.guest_driver_id
                      ? { name: 'guest-driver-detail', params: { id: s.guest_driver_id } }
                      : { name: 'driver-detail', params: { guid: s.guid } }"
                    target="_blank"
                    :title="s.guest_driver_id ? 'Open guest driver profile' : 'Open driver detail'"
                    class="grid size-8 shrink-0 place-items-center rounded-lg border border-line bg-surface-2/70 text-muted transition-colors hover:border-accent/60 hover:text-accent"
                  >
                    <Icon :name="s.guest_driver_id ? 'users' : 'user'" :size="14" />
                  </RouterLink>
                </div>
              </td>
            </tr>

            <tr v-if="!paged.length">
              <td colspan="5" class="px-3 py-10 text-center font-mono text-xs text-dim">
                {{ !loaded ? "Loading…" : hasFilters ? "No scores match these filters." : discipline === "drift" ? "No drift runs recorded yet." : "No timed laps recorded yet." }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <footer v-if="filtered.length" class="flex items-center justify-between gap-2 border-t border-line/60 px-4 py-2.5 font-mono text-xs text-dim">
        <span>{{ filtered.length }} result{{ filtered.length === 1 ? "" : "s" }}</span>
        <div class="flex items-center gap-1.5">
          <button
            type="button"
            class="inline-flex h-8 cursor-pointer items-center gap-1 rounded-md border border-line bg-surface-2/70 px-2.5 transition-colors hover:border-accent/60 hover:text-accent disabled:cursor-default disabled:opacity-40 disabled:hover:border-line disabled:hover:text-dim"
            :disabled="page <= 1"
            @click="page--"
          >
            <Icon name="arrowUp" :size="14" class="-rotate-90" /> Prev
          </button>
          <span class="px-1 tracking-wider text-muted">{{ page }} / {{ pageCount }}</span>
          <button
            type="button"
            class="inline-flex h-8 cursor-pointer items-center gap-1 rounded-md border border-line bg-surface-2/70 px-2.5 transition-colors hover:border-accent/60 hover:text-accent disabled:cursor-default disabled:opacity-40 disabled:hover:border-line disabled:hover:text-dim"
            :disabled="page >= pageCount"
            @click="page++"
          >
            Next <Icon name="arrowUp" :size="14" class="rotate-90" />
          </button>
        </div>
      </footer>
    </section>
  </div>

  <!-- ░░ Highlight-clip popup ░░ -->
  <Transition name="fade">
    <div
      v-if="clip"
      class="fixed inset-0 z-60 grid place-items-center bg-bg/80 p-4 backdrop-blur-sm"
      @click.self="clip = null"
    >
      <div class="flex w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-line bg-surface shadow-2xl">
        <header class="flex items-center gap-3 border-b border-line/60 px-4 py-3">
          <Icon name="film" :size="16" class="text-accent" />
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-bold">{{ clip.caption || "Highlight clip" }}</div>
            <div v-if="clip?.trigger" class="font-mono text-[11px] text-dim">
              {{ fmtScore(clip.trigger.drift_score) }} pts · +{{ fmtScore(clip.trigger.delta) }}
              <span v-if="clip?.trigger.track"> · {{ clip.trigger.track }}</span>
            </div>
          </div>
          <button
            type="button"
            class="grid size-8 shrink-0 place-items-center rounded-md border border-line bg-surface-2/70 text-muted transition-colors hover:border-accent/60 hover:text-accent disabled:opacity-40"
            title="Download clip"
            :disabled="!isRealMedia(clip)"
            @click="downloadClip(clip)"
          >
            <Icon name="download" :size="15" />
          </button>
          <button
            type="button"
            class="grid size-8 shrink-0 place-items-center rounded-md border border-line bg-surface-2/70 text-muted transition-colors hover:border-danger/60 hover:text-danger"
            title="Close"
            @click="clip = null"
          >
            <Icon name="x" :size="15" />
          </button>
        </header>
        <video
          v-if="isRealMedia(clip)"
          :src="clip?.url"
          class="aspect-video w-full bg-black"
          controls
          autoplay
          playsinline
        />
        <div v-else class="grid aspect-video w-full place-items-center bg-bg text-center">
          <div class="flex flex-col items-center gap-2 text-dim">
            <Icon name="film" :size="32" />
            <span class="font-mono text-[11px] tracking-wider uppercase">Clip preview unavailable</span>
          </div>
        </div>
      </div>
    </div>
  </Transition>

  <!-- ░░ Reassign-driver editor ░░ -->
  <Modal :open="!!editing" title="Set driver for this result" @close="editing = null">
    <div v-if="editing" class="space-y-4">
      <div class="flex items-center gap-3 rounded-md border border-line bg-surface-2/40 p-3">
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-bold">{{ editing.driver }}</div>
          <div class="truncate font-mono text-[11px] text-dim">
            {{ editing.car.name }} · {{ editing.track.name }} · {{ timeAgo(editing.date) }}
          </div>
        </div>
        <span
          class="numerals shrink-0 text-lg font-semibold"
          :class="editing?.kind === 'drift' ? 'text-accent' : 'text-ok'"
        >
          {{ editing.kind === "drift" ? fmtScore(editing.drift_score ?? 0) + " pts" : lapTime(editing.best_lap_ms ?? 0) }}
        </span>
      </div>
      <FormRow label="Driver" hint="Choose who actually set this result, or revert to the account's own name.">
        <UiSelect v-model="editSel" :options="extraOptions" />
      </FormRow>
      <p v-if="!roster.length" class="text-xs text-dim">
        No guest drivers yet — add them on the Guest Drivers page first.
      </p>
    </div>
    <template #footer>
      <Button variant="ghost" :disabled="savingEdit" @click="editing = null">Cancel</Button>
      <Button :disabled="savingEdit" @click="saveEdit">Save</Button>
    </template>
  </Modal>
</template>

<style scoped>
/* Distinctive broadcast display face for the big score/lap numerals; body text
   stays on the app's Plus Jakarta Sans. Scoped styles don't reach into child
   components, so the font-family is declared here for this component's own DOM.
   The @import is deduped by the browser when the host page also loads it. */
@import url("https://fonts.googleapis.com/css2?family=Saira+Condensed:wght@400;500;600;700&display=swap");

.numerals {
  font-family: "Saira Condensed", "Plus Jakarta Sans", sans-serif;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.01em;
}

.live-dot {
  animation: blink 1.1s steps(1, end) infinite;
}

@keyframes blink {
  0%,
  60% {
    opacity: 1;
  }
  61%,
  100% {
    opacity: 0.15;
  }
}
</style>
