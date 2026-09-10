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
