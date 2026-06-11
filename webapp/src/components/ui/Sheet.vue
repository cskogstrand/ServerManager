<script setup lang="ts">
// Responsive picker container: side panel on desktop, full-screen sheet on
// mobile. Replaces the old UI's separate mobile_*.htm picker pages.
import Icon from "@/components/ui/Icon.vue";

defineProps<{
  open: boolean;
  title?: string;
}>();

const emit = defineEmits<{ close: [] }>();
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="open" class="fixed inset-0 z-40 bg-black/55" @click="emit('close')" />
    </Transition>
    <Transition name="slide">
      <div
        v-if="open"
        role="dialog"
        aria-modal="true"
        class="fixed inset-y-0 right-0 z-50 flex w-full flex-col border-l border-line bg-surface shadow-2xl sm:max-w-md"
      >
        <header class="flex min-h-12 items-center border-b border-line bg-surface-2/35 px-4 py-3">
          <h2 class="text-sm font-bold">{{ title }}</h2>
          <button
            type="button"
            class="ml-auto inline-flex size-8 cursor-pointer items-center justify-center rounded-md border border-transparent text-muted transition-colors hover:border-line-hi hover:bg-surface-2 hover:text-text"
            aria-label="Close"
            @click="emit('close')"
          >
            <Icon name="x" :size="16" />
          </button>
        </header>
        <div class="min-h-0 flex-1 overflow-y-auto p-4">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="flex justify-end gap-2 border-t border-line px-4 py-3">
          <slot name="footer" />
        </footer>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
.slide-enter-active,
.slide-leave-active {
  transition: transform 0.2s ease;
}
.slide-enter-from,
.slide-leave-to {
  transform: translateX(100%);
}
</style>
