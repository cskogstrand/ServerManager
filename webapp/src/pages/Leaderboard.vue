<script setup lang="ts">
// Standalone all-servers leaderboard — the same broadcast graphics surface as
// the auto-follow fallback, but on its own route + menu item so it stays
// reachable even while a server is populated. Full-screen (bare) for a second
// screen; the content + clip popup live in <BroadcastLeaderboard>.
import BroadcastLeaderboard from "@/components/BroadcastLeaderboard.vue";
import Icon from "@/components/ui/Icon.vue";

function toggleFullscreen() {
  if (document.fullscreenElement) void document.exitFullscreen();
  else void document.documentElement.requestFullscreen().catch(() => {});
}
</script>

<template>
  <div class="bcast fixed inset-0 z-50 overflow-hidden bg-bg text-text select-none">
    <!-- Atmosphere layers -->
    <div class="bcast-bg pointer-events-none absolute inset-0" />
    <div class="bcast-scan pointer-events-none absolute inset-0" />
    <div class="bcast-vignette pointer-events-none absolute inset-0" />

    <!-- ░░ Top strap ░░ -->
    <header class="absolute inset-x-0 top-0 z-30 flex h-16 items-center gap-2 px-3 sm:gap-4 sm:px-5">
      <div class="flex min-w-0 items-center gap-2.5">
        <span class="inline-flex items-center gap-1.5 rounded-sm bg-accent px-2 py-1 text-xs font-black tracking-[0.2em] text-bg">
          <Icon name="trophy" :size="12" />
          LEADERBOARD
        </span>
        <div class="min-w-0 leading-tight">
          <div class="truncate text-sm font-extrabold tracking-tight">All Servers</div>
          <div class="font-mono text-[11px] text-dim">drift bests · best laps</div>
        </div>
      </div>

      <div class="ml-auto flex items-center gap-1.5">
        <button
          type="button"
          class="grid size-8 place-items-center sm:size-9 rounded-md border border-line bg-surface/70 text-muted transition-colors hover:border-line-hi hover:text-text"
          title="Toggle fullscreen"
          @click="toggleFullscreen"
        >
          <Icon name="maximize" :size="16" />
        </button>
        <RouterLink
          to="/"
          class="grid size-8 place-items-center sm:size-9 rounded-md border border-line bg-surface/70 text-muted transition-colors hover:border-danger/60 hover:text-danger"
          title="Exit leaderboard"
        >
          <Icon name="x" :size="16" />
        </RouterLink>
      </div>
    </header>

    <!-- ░░ Stage ░░ -->
    <div class="absolute inset-x-0 bottom-0 top-16 z-10 px-3 pb-3 lg:px-4 lg:pb-4">
      <BroadcastLeaderboard />
    </div>
  </div>
</template>

<style scoped>
/* Distinctive broadcast display face for big numerals/timers; body text stays
   on the app's Plus Jakarta Sans for cohesion. */
@import url("https://fonts.googleapis.com/css2?family=Saira+Condensed:wght@400;500;600;700&display=swap");

.numerals {
  font-family: "Saira Condensed", "Plus Jakarta Sans", sans-serif;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.01em;
}

/* Blueprint grid + depth glow behind the stage. */
.bcast-bg {
  background-color: var(--color-bg);
  background-image: radial-gradient(ellipse 80% 60% at 50% 18%, rgba(98, 179, 232, 0.1), transparent 70%),
    linear-gradient(var(--color-line) 1px, transparent 1px),
    linear-gradient(90deg, var(--color-line) 1px, transparent 1px);
  background-size: 100% 100%, 44px 44px, 44px 44px;
  background-position: 0 0, -1px -1px, -1px -1px;
  opacity: 0.9;
}

/* Cinematic vignette so the corners fall off behind the overlays. */
.bcast-vignette {
  background: radial-gradient(ellipse 75% 75% at 50% 45%, transparent 55%, rgba(0, 0, 0, 0.55) 100%);
}

/* Faint broadcast scanlines, drifting slowly. */
.bcast-scan {
  background: repeating-linear-gradient(
    to bottom,
    rgba(255, 255, 255, 0.018) 0px,
    rgba(255, 255, 255, 0.018) 1px,
    transparent 2px,
    transparent 4px
  );
  animation: scan 14s linear infinite;
  mix-blend-mode: overlay;
}

@keyframes scan {
  from {
    background-position-y: 0;
  }
  to {
    background-position-y: 200px;
  }
}
</style>
