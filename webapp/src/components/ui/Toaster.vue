<script setup lang="ts">
// Global toast stack, mounted once in App.vue. Bottom-right on desktop,
// top on mobile (clear of the bottom tab bar).
import { useRouter } from "vue-router";
import { useToastStore } from "@/stores/toast";
import { activeDialog } from "@/lib/useDialog";
import Icon from "@/components/ui/Icon.vue";

const toasts = useToastStore();
const router = useRouter();

function act(id: number, to: string) {
  toasts.dismiss(id);
  void router.push(to);
}

const tone = {
  success: { icon: "check", cls: "border-ok/45 bg-surface text-ok" },
  error: { icon: "alert", cls: "border-danger/45 bg-surface text-danger" },
  info: { icon: "info", cls: "border-accent/45 bg-surface text-text" },
} as const;
</script>

<template>
  <Teleport :to="activeDialog ?? 'body'">
    <div
      class="pointer-events-none fixed inset-x-0 top-3 z-[60] flex flex-col items-center gap-2 px-3 sm:inset-x-auto sm:right-4 sm:bottom-4 sm:top-auto sm:items-end"
    >
      <TransitionGroup name="toast">
        <div
          v-for="t in toasts.toasts"
          :key="t.id"
          class="pointer-events-auto flex w-full max-w-sm items-start gap-2.5 rounded-md border px-3 py-2.5 text-sm shadow-lg backdrop-blur"
          :class="tone[t.tone].cls"
          :role="t.tone === 'error' ? 'alert' : 'status'"
        >
          <Icon :name="tone[t.tone].icon" :size="17" class="mt-0.5 shrink-0" />
          <div class="flex min-w-0 flex-1 flex-col gap-1.5">
            <span class="break-words">{{ t.message }}</span>
            <button
              v-if="t.action"
              type="button"
              class="min-h-11 self-start cursor-pointer rounded border border-current/40 px-2 py-0.5 text-xs font-semibold opacity-90 transition-opacity hover:opacity-100"
              @click="act(t.id, t.action.to)"
            >
              {{ t.action.label }} →
            </button>
          </div>
          <button
            type="button"
            class="-mr-1 flex size-11 shrink-0 cursor-pointer items-center justify-center rounded text-muted hover:bg-surface-2"
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
