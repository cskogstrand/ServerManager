<script setup lang="ts">
import { toRef, useId } from "vue";
import { useDialog } from "@/lib/useDialog";
import Icon from "@/components/ui/Icon.vue";

const props = defineProps<{ open: boolean; title?: string; wide?: boolean }>();
const emit = defineEmits<{ close: [] }>();
const dialog = useDialog(toRef(props, "open"));
const titleId = useId();
</script>

<template>
  <Teleport to="body">
    <dialog ref="dialog" :aria-labelledby="titleId" class="app-dialog app-sheet" @cancel.prevent="emit('close')" @click.self="emit('close')">
      <div v-if="open" class="flex h-dvh w-full flex-col border-l border-line bg-surface text-text shadow-2xl" :class="wide ? 'sm:max-w-5xl' : 'sm:max-w-md'">
        <header class="flex items-center gap-3 border-b border-line px-5 py-3">
          <h2 :id="titleId" class="text-lg font-semibold">{{ title || 'Details' }}</h2>
          <button type="button" class="ml-auto grid size-11 shrink-0 cursor-pointer place-items-center rounded-md text-muted hover:bg-surface-2 hover:text-text" aria-label="Close panel" @click="emit('close')"><Icon name="x" :size="18" /></button>
        </header>
        <div class="min-h-0 flex-1 overflow-y-auto p-5"><slot /></div>
        <footer v-if="$slots.footer" class="flex flex-wrap items-center justify-end gap-2 border-t border-line px-5 pt-4 pb-[max(1rem,env(safe-area-inset-bottom))]"><slot name="footer" /></footer>
      </div>
    </dialog>
  </Teleport>
</template>
