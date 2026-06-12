<script setup lang="ts">
// Global toast stack, mounted once in App.vue. Bottom-right on desktop,
// top on mobile (clear of the bottom tab bar).
import { useToastStore } from "@/stores/toast";
import Icon from "@/components/ui/Icon.vue";

const toasts = useToastStore();

const tone = {
  success: { icon: "check", cls: "border-ok/45 bg-ok-glow text-ok" },
  error: { icon: "alert", cls: "border-danger/45 bg-danger-glow text-danger" },
  info: { icon: "info", cls: "border-accent/45 bg-accent-dim text-text" },
} as const;
</script>

<template>
  <Teleport to="body">
    <div
      class="pointer-events-none fixed inset-x-0 top-3 z-[60] flex flex-col items-center gap-2 px-3 sm:inset-x-auto sm:right-4 sm:bottom-4 sm:top-auto sm:items-end"
    >
      <TransitionGroup name="toast">
        <div
          v-for="t in toasts.toasts"
          :key="t.id"
          class="pointer-events-auto flex w-full max-w-sm items-start gap-2.5 rounded-md border px-3 py-2.5 text-sm shadow-lg backdrop-blur"
          :class="tone[t.tone].cls"
          role="status"
        >
          <Icon :name="tone[t.tone].icon" :size="17" class="mt-0.5 shrink-0" />
          <span class="min-w-0 flex-1 break-words">{{ t.message }}</span>
          <button
            type="button"
            class="-mr-1 shrink-0 cursor-pointer rounded p-0.5 opacity-60 transition-opacity hover:opacity-100"
            aria-label="Dismiss"
            @click="toasts.dismiss(t.id)"
          >
            <Icon name="x" :size="14" />
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.2s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
.toast-leave-active {
  position: absolute;
}
</style>
