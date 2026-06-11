<script setup lang="ts">
// Port of pw-modal: overlay + centered panel with head/body/footer.
// One component replaces the 8+ copy-pasted modal markups of the old UI.
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
    <Transition name="pop">
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
        @click.self="emit('close')"
      >
        <div
          role="dialog"
          aria-modal="true"
          class="flex max-h-[85vh] w-full max-w-lg flex-col rounded-md border border-line bg-surface shadow-2xl"
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
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active,
.pop-enter-active,
.pop-leave-active {
  transition: all 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
.pop-enter-from,
.pop-leave-to {
  opacity: 0;
  transform: scale(0.97);
}
</style>
