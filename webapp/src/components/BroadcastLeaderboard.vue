<script setup lang="ts">
// All-servers leaderboard: drift bests and best laps aggregated across every
// driver the backend knows, in the broadcast graphics style. Self-contained —
// loads its own data on mount and refreshes on a slow timer so it can stand as
// a permanent trackside display. A drift score with a linked highlight clip
// opens it in a modal with a download button.
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { listDrivers, fmtScore } from "@/lib/driversApi";
import { lapTime } from "@/lib/raceTelemetry";
import type { DriverSummary, MediaItem } from "@/types/driverStats";
import Icon from "@/components/ui/Icon.vue";

const allDrivers = ref<DriverSummary[]>([]);
const leaderLoaded = ref(false);

async function loadLeaderboard() {
  try {
    allDrivers.value = await listDrivers();
  } catch {
    allDrivers.value = [];
  } finally {
    leaderLoaded.value = true;
  }
}

const topDrift = computed(() =>
  [...allDrivers.value]
    .filter((d) => d.best_drift > 0)
    .sort((a, b) => b.best_drift - a.best_drift)
    .slice(0, 12),
);
const topLap = computed(() =>
  [...allDrivers.value]
    .filter((d) => d.best_lap_ms > 0)
    .sort((a, b) => a.best_lap_ms - b.best_lap_ms)
    .slice(0, 12),
);

// --- Highlight-clip popup --------------------------------------------------
// A drift score with a linked clip opens it in a modal with a download button.
// Real captures resolve to a served file; mock items ("#") show a placeholder.
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

// Slow refresh keeps a standing display current without heavy polling.
let poll: ReturnType<typeof setInterval> | null = null;
onMounted(() => {
  void loadLeaderboard();
  poll = setInterval(() => void loadLeaderboard(), 30000);
});
onBeforeUnmount(() => {
  if (poll) clearInterval(poll);
});
</script>

<template>
  <div class="flex h-full w-full flex-col gap-3 overflow-y-auto lg:flex-row lg:gap-4">
    <!-- Top drift -->
    <section class="flex w-full min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-line bg-surface/30 backdrop-blur-sm">
      <header class="flex items-center gap-2 border-b border-line/60 px-4 py-3">
        <Icon name="broadcast" :size="16" class="text-accent" />
        <h2 class="numerals text-lg tracking-tight">Top Drift</h2>
        <span class="ml-auto font-mono text-[11px] tracking-wider text-dim uppercase">all servers</span>
      </header>
      <ol class="flex-1 overflow-y-auto p-2">
        <li
          v-for="(d, i) in topDrift"
          :key="d.guid"
          class="flex items-center gap-3 rounded-lg px-3 py-2.5"
          :class="i === 0 ? 'bg-accent-glow/5' : i % 2 ? 'bg-surface/30' : ''"
        >
          <span
            class="numerals grid size-8 shrink-0 place-items-center rounded-lg text-base font-bold tabular-nums"
            :class="i === 0 ? 'bg-accent text-bg' : 'bg-surface-3 text-muted'"
          >{{ i + 1 }}</span>
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-bold">{{ d.name }}</div>
            <div class="truncate font-mono text-[11px] text-dim">{{ d.favourite_track?.name || d.favourite_car?.name || "—" }}</div>
          </div>
          <span class="numerals text-xl font-semibold tabular-nums text-accent">{{ fmtScore(d.best_drift) }}</span>
          <span class="font-mono text-[10px] tracking-wider text-dim uppercase">pts</span>
          <button
            v-if="d.best_drift_clip"
            type="button"
            class="grid size-8 shrink-0 place-items-center rounded-lg border border-line bg-surface-2/70 text-muted transition-colors hover:border-accent/60 hover:text-accent"
            title="Watch the highlight clip from this run"
            @click="clip = d.best_drift_clip ?? null"
          >
            <Icon name="film" :size="15" />
          </button>
        </li>
        <li v-if="!topDrift.length" class="px-3 py-6 text-center font-mono text-xs text-dim">
          {{ leaderLoaded ? "No drift scores recorded yet." : "Loading…" }}
        </li>
      </ol>
    </section>

    <!-- Best laps -->
    <section class="flex w-full min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-line bg-surface/30 backdrop-blur-sm">
      <header class="flex items-center gap-2 border-b border-line/60 px-4 py-3">
        <Icon name="activity" :size="16" class="text-ok" />
        <h2 class="numerals text-lg tracking-tight">Best Laps</h2>
        <span class="ml-auto font-mono text-[11px] tracking-wider text-dim uppercase">all servers</span>
      </header>
      <ol class="flex-1 overflow-y-auto p-2">
        <li
          v-for="(d, i) in topLap"
          :key="d.guid"
          class="flex items-center gap-3 rounded-lg px-3 py-2.5"
          :class="i === 0 ? 'bg-accent-glow/5' : i % 2 ? 'bg-surface/30' : ''"
        >
          <span
            class="numerals grid size-8 shrink-0 place-items-center rounded-lg text-base font-bold tabular-nums"
            :class="i === 0 ? 'bg-ok text-bg' : 'bg-surface-3 text-muted'"
          >{{ i + 1 }}</span>
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-bold">{{ d.name }}</div>
            <div class="truncate font-mono text-[11px] text-dim">{{ d.favourite_track?.name || d.favourite_car?.name || "—" }}</div>
          </div>
          <span class="numerals text-xl font-semibold tabular-nums text-ok">{{ lapTime(d.best_lap_ms) }}</span>
        </li>
        <li v-if="!topLap.length" class="px-3 py-6 text-center font-mono text-xs text-dim">
          {{ leaderLoaded ? "No lap times recorded yet." : "Loading…" }}
        </li>
      </ol>
    </section>
  </div>

  <!-- ░░ Highlight-clip popup ░░ -->
  <Transition name="fade">
    <div
      v-if="clip"
      class="fixed inset-0 z-[60] grid place-items-center bg-bg/80 p-4 backdrop-blur-sm"
      @click.self="clip = null"
    >
      <div class="flex w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-line bg-surface shadow-2xl">
        <header class="flex items-center gap-3 border-b border-line/60 px-4 py-3">
          <Icon name="film" :size="16" class="text-accent" />
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-bold">{{ clip.caption || "Highlight clip" }}</div>
            <div v-if="clip.trigger" class="font-mono text-[11px] text-dim">
              {{ fmtScore(clip.trigger.drift_score) }} pts · +{{ fmtScore(clip.trigger.delta) }}
              <span v-if="clip.trigger.track"> · {{ clip.trigger.track }}</span>
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
          :src="clip.url"
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
</template>
